package onlineexperience

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type Handler struct {
	apiKeyService       *service.APIKeyService
	subscriptionService *service.SubscriptionService
	gatewayService      *service.GatewayService
	cfg                 *config.Config
}

type Model struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
}

func NewHandler(
	apiKeyService *service.APIKeyService,
	subscriptionService *service.SubscriptionService,
	gatewayService *service.GatewayService,
	cfg *config.Config,
) *Handler {
	return &Handler{
		apiKeyService:       apiKeyService,
		subscriptionService: subscriptionService,
		gatewayService:      gatewayService,
		cfg:                 cfg,
	}
}

func (h *Handler) ListGroups(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	groups, err := h.apiKeyService.GetAvailableGroups(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	available := make([]dto.Group, 0, len(groups))
	for i := range groups {
		if groups[i].Platform == service.PlatformOpenAI {
			available = append(available, *dto.GroupFromService(&groups[i]))
		}
	}

	response.Success(c, available)
}

func (h *Handler) ListModels(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	groupID, err := parseGroupID(strings.TrimSpace(c.Query("group_id")))
	if err != nil {
		response.BadRequest(c, "Invalid group_id")
		return
	}

	group, err := h.resolveOpenAIGroup(c.Request.Context(), subject.UserID, groupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	modelIDs := h.gatewayService.GetAvailableModels(c.Request.Context(), &group.ID, "")
	models := buildModelsResponse(modelIDs)
	if len(models) == 0 {
		models = buildModelsResponse(openai.DefaultModelIDs())
	}

	response.Success(c, models)
}

func (h *Handler) BindGroupContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := middleware2.GetAuthSubjectFromContext(c)
		if !ok {
			writeOpenAIError(c, http.StatusUnauthorized, "authentication_error", "User not authenticated")
			c.Abort()
			return
		}

		groupID, err := h.extractGroupIDFromRequest(c)
		if err != nil {
			writeOpenAIError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
			c.Abort()
			return
		}

		apiKey, err := h.apiKeyService.EnsureOnlineExperienceAPIKey(c.Request.Context(), subject.UserID, groupID)
		if err != nil {
			if errors.Is(err, service.ErrGroupNotAllowed) {
				writeOpenAIError(c, http.StatusForbidden, "permission_error", "Selected group is not available")
			} else {
				writeOpenAIError(c, http.StatusInternalServerError, "api_error", "Failed to prepare online experience context")
			}
			c.Abort()
			return
		}

		if apiKey.User == nil {
			writeOpenAIError(c, http.StatusUnauthorized, "authentication_error", "User associated with API key not found")
			c.Abort()
			return
		}
		if !apiKey.User.IsActive() {
			writeOpenAIError(c, http.StatusUnauthorized, "authentication_error", "User account is not active")
			c.Abort()
			return
		}
		if !apiKey.IsActive() &&
			apiKey.Status != service.StatusAPIKeyExpired &&
			apiKey.Status != service.StatusAPIKeyQuotaExhausted {
			writeOpenAIError(c, http.StatusUnauthorized, "authentication_error", "API key is disabled")
			c.Abort()
			return
		}

		if h.cfg != nil && h.cfg.RunMode == config.RunModeSimple {
			h.setGatewayContext(c, apiKey, nil)
			c.Next()
			return
		}

		var subscription *service.UserSubscription
		if apiKey.Group != nil && apiKey.Group.IsSubscriptionType() && h.subscriptionService != nil {
			sub, subErr := h.subscriptionService.GetActiveSubscription(c.Request.Context(), apiKey.User.ID, apiKey.Group.ID)
			if subErr != nil {
				writeOpenAIError(c, http.StatusForbidden, "permission_error", "No active subscription found for this group")
				c.Abort()
				return
			}
			needsMaintenance, validateErr := h.subscriptionService.ValidateAndCheckLimits(sub, apiKey.Group)
			if validateErr != nil {
				status := http.StatusForbidden
				if validateErr == service.ErrDailyLimitExceeded || validateErr == service.ErrWeeklyLimitExceeded || validateErr == service.ErrMonthlyLimitExceeded {
					status = http.StatusTooManyRequests
				}
				writeOpenAIError(c, status, "permission_error", validateErr.Error())
				c.Abort()
				return
			}
			if needsMaintenance {
				maintenanceCopy := *sub
				h.subscriptionService.DoWindowMaintenance(&maintenanceCopy)
			}
			subscription = sub
		} else {
			if apiKey.User.Balance <= 0 {
				writeOpenAIError(c, http.StatusForbidden, "permission_error", "Insufficient account balance")
				c.Abort()
				return
			}
		}

		if apiKey.Status == service.StatusAPIKeyQuotaExhausted || apiKey.IsQuotaExhausted() {
			writeOpenAIError(c, http.StatusTooManyRequests, "permission_error", "API key quota exhausted")
			c.Abort()
			return
		}
		if apiKey.Status == service.StatusAPIKeyExpired || apiKey.IsExpired() {
			writeOpenAIError(c, http.StatusForbidden, "permission_error", "API key expired")
			c.Abort()
			return
		}

		h.setGatewayContext(c, apiKey, subscription)
		c.Next()
	}
}

func (h *Handler) setGatewayContext(c *gin.Context, apiKey *service.APIKey, subscription *service.UserSubscription) {
	if subscription != nil {
		c.Set(string(middleware2.ContextKeySubscription), subscription)
	}
	c.Set(string(middleware2.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{
		UserID:      apiKey.User.ID,
		Concurrency: apiKey.User.Concurrency,
	})
	c.Set(string(middleware2.ContextKeyUserRole), apiKey.User.Role)

	if group := apiKey.Group; service.IsGroupContextValid(group) {
		if existing, ok := c.Request.Context().Value(ctxkey.Group).(*service.Group); !ok || existing == nil || existing.ID != group.ID || !service.IsGroupContextValid(existing) {
			ctx := context.WithValue(c.Request.Context(), ctxkey.Group, group)
			c.Request = c.Request.WithContext(ctx)
		}
	}

	_ = h.apiKeyService.TouchLastUsed(c.Request.Context(), apiKey.ID)
}

func (h *Handler) resolveOpenAIGroup(ctx context.Context, userID, groupID int64) (*service.Group, error) {
	groups, err := h.apiKeyService.GetAvailableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		if groups[i].ID == groupID && groups[i].Platform == service.PlatformOpenAI {
			group := groups[i]
			return &group, nil
		}
	}
	return nil, service.ErrGroupNotAllowed
}

func (h *Handler) extractGroupIDFromRequest(c *gin.Context) (int64, error) {
	if rawGroupID := strings.TrimSpace(c.Query("group_id")); rawGroupID != "" {
		return parseGroupID(rawGroupID)
	}

	if c.Request == nil || c.Request.Body == nil {
		return 0, fmt.Errorf("group_id is required")
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read request body")
	}

	contentType := strings.TrimSpace(c.GetHeader("Content-Type"))
	mediaType, _, _ := mime.ParseMediaType(contentType)

	if strings.EqualFold(mediaType, "multipart/form-data") {
		groupID, groupErr := extractGroupIDFromMultipart(body, contentType)
		restoreRequestBody(c, body)
		if groupErr != nil {
			return 0, groupErr
		}
		return groupID, nil
	}

	if len(body) == 0 {
		restoreRequestBody(c, body)
		return 0, fmt.Errorf("group_id is required")
	}
	if !gjson.ValidBytes(body) {
		restoreRequestBody(c, body)
		return 0, fmt.Errorf("failed to parse request body")
	}

	groupResult := gjson.GetBytes(body, "group_id")
	if !groupResult.Exists() {
		restoreRequestBody(c, body)
		return 0, fmt.Errorf("group_id is required")
	}

	groupID, err := parseJSONGroupID(groupResult)
	if err != nil {
		restoreRequestBody(c, body)
		return 0, err
	}

	sanitizedBody := body
	if updated, deleteErr := sjson.DeleteBytes(body, "group_id"); deleteErr == nil {
		sanitizedBody = updated
	}
	restoreRequestBody(c, sanitizedBody)
	return groupID, nil
}

func buildModelsResponse(modelIDs []string) []Model {
	if len(modelIDs) == 0 {
		return nil
	}

	byID := make(map[string]openai.Model, len(openai.DefaultModels))
	for _, item := range openai.DefaultModels {
		byID[item.ID] = item
	}

	out := make([]Model, 0, len(modelIDs))
	seen := make(map[string]struct{}, len(modelIDs))
	for _, modelID := range modelIDs {
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			continue
		}
		if _, exists := seen[modelID]; exists {
			continue
		}
		seen[modelID] = struct{}{}

		item := Model{
			ID:          modelID,
			DisplayName: modelID,
			Type:        "chat",
		}
		if meta, ok := byID[modelID]; ok && strings.TrimSpace(meta.DisplayName) != "" {
			item.DisplayName = meta.DisplayName
		}
		if strings.HasPrefix(strings.ToLower(modelID), "gpt-image-") {
			item.Type = "image"
		}
		out = append(out, item)
	}

	return out
}

func parseGroupID(raw string) (int64, error) {
	groupID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || groupID <= 0 {
		return 0, fmt.Errorf("invalid group_id")
	}
	return groupID, nil
}

func parseJSONGroupID(result gjson.Result) (int64, error) {
	switch result.Type {
	case gjson.Number:
		if result.Int() <= 0 {
			return 0, fmt.Errorf("invalid group_id")
		}
		return result.Int(), nil
	case gjson.String:
		return parseGroupID(strings.TrimSpace(result.String()))
	default:
		return 0, fmt.Errorf("invalid group_id")
	}
}

func extractGroupIDFromMultipart(body []byte, contentType string) (int64, error) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return 0, fmt.Errorf("invalid multipart content-type")
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return 0, fmt.Errorf("multipart boundary is required")
	}

	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("failed to parse multipart body")
		}
		if strings.TrimSpace(part.FileName()) != "" {
			_ = part.Close()
			continue
		}

		name := strings.TrimSpace(part.FormName())
		value, readErr := io.ReadAll(part)
		_ = part.Close()
		if readErr != nil {
			return 0, fmt.Errorf("failed to read multipart field")
		}
		if name != "group_id" {
			continue
		}
		return parseGroupID(strings.TrimSpace(string(value)))
	}

	return 0, fmt.Errorf("group_id is required")
}

func restoreRequestBody(c *gin.Context, body []byte) {
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
}

func writeOpenAIError(c *gin.Context, statusCode int, errorType string, message string) {
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"type":    errorType,
			"message": message,
		},
	})
}
