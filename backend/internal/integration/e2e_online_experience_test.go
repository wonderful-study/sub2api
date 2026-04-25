//go:build e2e

package integration

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// 在线体验 E2E 用户旅程：
// 1. 普通用户用 JWT 访问在线体验，不需要手动创建 API Key。
// 2. 用户只能看到可用的 OpenAI 分组，并基于分组加载模型。
// 3. 对话、文生图、图生图请求都会进入在线体验兼容入口，再复用现有网关上下文。
// 4. 在线体验自动创建的内部 API Key 不应出现在普通用户 API Key 列表中。

const onlineExperienceInternalKeyPrefix = "__online_experience__"

type onlineExperienceGroup struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Status   string `json:"status"`
}

type onlineExperienceModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
}

type onlineExperienceAPIKey struct {
	ID   int64  `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type onlineExperiencePaginatedKeys struct {
	Items []onlineExperienceAPIKey `json:"items"`
	Total int64                    `json:"total"`
}

type onlineExperienceErrorResponse struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
	Message string `json:"message"`
}

func TestOnlineExperienceRejectsUnauthenticatedUser(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{
			name:   "分组列表需要登录",
			method: http.MethodGet,
			path:   "/api/v1/online-experience/groups",
		},
		{
			name:   "对话请求需要登录",
			method: http.MethodPost,
			path:   "/api/v1/online-experience/chat",
			body:   []byte(`{"group_id":1,"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}]}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := doRequest(t, tt.method, tt.path, tt.body, "")
			if err != nil {
				t.Skipf("后端服务不可用，跳过在线体验未登录校验: %v", err)
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("期望 HTTP 401，得到 HTTP %d: %s", resp.StatusCode, string(body))
			}
		})
	}
}

func TestOnlineExperienceRequiresGroupID(t *testing.T) {
	token := loginOnlineExperienceUser(t)

	payload := map[string]any{
		"model": "gpt-5.4",
		"messages": []map[string]string{
			{"role": "user", "content": "Say hello."},
		},
		"stream": true,
	}
	body, _ := json.Marshal(payload)

	resp, err := doRequest(t, http.MethodPost, "/api/v1/online-experience/chat", body, token)
	if err != nil {
		t.Fatalf("在线体验对话请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("缺少 group_id 应返回 HTTP 400，得到 HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var errResp onlineExperienceErrorResponse
	if err := json.Unmarshal(respBody, &errResp); err != nil {
		t.Fatalf("解析 OpenAI 兼容错误响应失败: %v, body=%s", err, string(respBody))
	}
	if !strings.Contains(strings.ToLower(errResp.Error.Message), "group_id") {
		t.Fatalf("错误信息应包含 group_id，实际为: %s", string(respBody))
	}
}

func TestOnlineExperienceGroupJSONContract(t *testing.T) {
	token := loginOnlineExperienceUser(t)

	assertOnlineExperienceGroupJSONContract(t, token)
}

func TestOnlineExperienceUserJourney(t *testing.T) {
	token := loginOnlineExperienceUser(t)

	assertOnlineExperienceGroupJSONContract(t, token)

	groups := onlineExperienceGETData[[]onlineExperienceGroup](t, "/api/v1/online-experience/groups", token)
	if len(groups) == 0 {
		t.Skip("当前用户没有可用 OpenAI 分组，跳过在线体验完整旅程测试")
	}

	group := selectOnlineExperienceGroup(t, groups)
	t.Logf("使用在线体验分组: id=%d name=%q platform=%q", group.ID, group.Name, group.Platform)

	models := onlineExperienceGETData[[]onlineExperienceModel](
		t,
		"/api/v1/online-experience/models?group_id="+url.QueryEscape(strconv.FormatInt(group.ID, 10)),
		token,
	)
	if len(models) == 0 {
		t.Fatalf("在线体验模型列表为空，group_id=%d", group.ID)
	}

	chatModel := firstOnlineExperienceModel(models, "chat")
	imageModel := firstOnlineExperienceModel(models, "image")
	t.Logf("在线体验模型: total=%d chat=%q image=%q", len(models), chatModel, imageModel)

	gatewayAttempted := false

	if chatModel != "" {
		t.Run("对话请求进入共享网关上下文", func(t *testing.T) {
			gatewayAttempted = true
			payload := map[string]any{
				"group_id": group.ID,
				"model":    chatModel,
				"messages": []map[string]string{
					{"role": "user", "content": "Say hello in one short sentence."},
				},
				"stream": true,
			}
			body, _ := json.Marshal(payload)
			resp, err := doOnlineExperienceJSONRequest(t, "/api/v1/online-experience/chat", body, token)
			if err != nil {
				t.Fatalf("在线体验对话请求失败: %v", err)
			}
			defer resp.Body.Close()

			respBody := assertOnlineExperienceGatewayResponse(t, "chat", resp)
			if resp.StatusCode == http.StatusOK && !bytes.Contains(respBody, []byte("data:")) {
				t.Fatalf("流式对话响应应包含 SSE data 行，实际响应: %s", string(respBody))
			}
		})
	} else {
		t.Log("模型列表中没有 chat 类型模型，跳过在线体验对话请求")
	}

	if imageModel == "" {
		t.Fatalf("在线体验模型列表中没有 image 类型模型，无法覆盖文生图和图生图场景")
	}

	t.Run("文生图请求进入共享网关上下文", func(t *testing.T) {
		gatewayAttempted = true
		payload := map[string]any{
			"group_id": group.ID,
			"model":    imageModel,
			"prompt":   "A small blue cube on a clean desk, product photo style.",
			"size":     "1024x1024",
			"n":        1,
		}
		body, _ := json.Marshal(payload)
		resp, err := doOnlineExperienceJSONRequest(t, "/api/v1/online-experience/images/generations", body, token)
		if err != nil {
			t.Fatalf("在线体验文生图请求失败: %v", err)
		}
		defer resp.Body.Close()

		respBody := assertOnlineExperienceGatewayResponse(t, "image generation", resp)
		if resp.StatusCode == http.StatusOK {
			assertOnlineExperienceImageData(t, respBody)
		}
	})

	t.Run("图生图请求进入共享网关上下文", func(t *testing.T) {
		gatewayAttempted = true
		body, contentType := buildOnlineExperienceImageEditBody(t, group.ID, imageModel)
		resp, err := doOnlineExperienceMultipartRequest(t, "/api/v1/online-experience/images/edits", body, contentType, token)
		if err != nil {
			t.Fatalf("在线体验图生图请求失败: %v", err)
		}
		defer resp.Body.Close()

		respBody := assertOnlineExperienceGatewayResponse(t, "image edit", resp)
		if resp.StatusCode == http.StatusOK {
			assertOnlineExperienceImageData(t, respBody)
		}
	})

	if !gatewayAttempted {
		t.Fatalf("模型列表中没有可用于在线体验请求的 chat 或 image 模型")
	}

	t.Run("内部 API Key 不暴露在普通密钥列表", func(t *testing.T) {
		keys := onlineExperienceGETData[onlineExperiencePaginatedKeys](
			t,
			"/api/v1/keys?search="+url.QueryEscape(onlineExperienceInternalKeyPrefix),
			token,
		)
		if keys.Total != 0 || len(keys.Items) != 0 {
			t.Fatalf("普通密钥列表不应暴露在线体验内部 key: total=%d items=%+v", keys.Total, keys.Items)
		}

		allKeys := onlineExperienceGETData[onlineExperiencePaginatedKeys](t, "/api/v1/keys?page_size=100", token)
		for _, key := range allKeys.Items {
			if strings.HasPrefix(key.Name, onlineExperienceInternalKeyPrefix) || strings.HasPrefix(key.Key, onlineExperienceInternalKeyPrefix) {
				t.Fatalf("普通密钥列表包含在线体验内部 key: %+v", key)
			}
		}
	})
}

func loginOnlineExperienceUser(t *testing.T) string {
	t.Helper()

	type credential struct {
		email    string
		password string
		source   string
	}

	candidates := []credential{
		{
			email:    strings.TrimSpace(os.Getenv("ONLINE_EXPERIENCE_USER_EMAIL")),
			password: os.Getenv("ONLINE_EXPERIENCE_USER_PASSWORD"),
			source:   "ONLINE_EXPERIENCE_USER_*",
		},
		{
			email:    strings.TrimSpace(os.Getenv("E2E_USER_EMAIL")),
			password: os.Getenv("E2E_USER_PASSWORD"),
			source:   "E2E_USER_*",
		},
		{
			email:    strings.TrimSpace(os.Getenv("ADMIN_EMAIL")),
			password: os.Getenv("ADMIN_PASSWORD"),
			source:   "ADMIN_*",
		},
		{
			email:    testUserEmail,
			password: testUserPassword,
			source:   "内置测试用户",
		},
	}

	var attempted bool
	var lastStatus int
	var lastBody string
	for _, item := range candidates {
		if item.email == "" || item.password == "" {
			continue
		}
		attempted = true

		payload := map[string]string{
			"email":    item.email,
			"password": item.password,
		}
		body, _ := json.Marshal(payload)
		resp, err := doRequest(t, http.MethodPost, "/api/v1/auth/login", body, "")
		if err != nil {
			t.Skipf("后端服务不可用，无法登录在线体验测试用户: %v", err)
		}

		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		lastStatus = resp.StatusCode
		lastBody = string(respBody)

		if resp.StatusCode != http.StatusOK {
			t.Logf("使用 %s 登录失败: HTTP %d", item.source, resp.StatusCode)
			continue
		}

		token := extractOnlineExperienceAccessToken(respBody)
		if token == "" {
			t.Logf("使用 %s 登录成功但响应中没有 access_token: %s", item.source, string(respBody))
			continue
		}
		t.Logf("使用 %s 登录在线体验 E2E 用户成功", item.source)
		return token
	}

	if !attempted {
		t.Skip("未配置在线体验 E2E 用户凭据，请设置 ONLINE_EXPERIENCE_USER_EMAIL 和 ONLINE_EXPERIENCE_USER_PASSWORD")
	}
	t.Skipf("无法登录在线体验 E2E 用户，最后一次响应 HTTP %d: %s", lastStatus, lastBody)
	return ""
}

func extractOnlineExperienceAccessToken(body []byte) string {
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return ""
	}
	if token, ok := result["access_token"].(string); ok {
		return token
	}
	if data, ok := result["data"].(map[string]any); ok {
		if token, ok := data["access_token"].(string); ok {
			return token
		}
	}
	return ""
}

func onlineExperienceGETData[T any](t *testing.T, path, token string) T {
	t.Helper()

	resp, err := doRequest(t, http.MethodGet, path, nil, token)
	if err != nil {
		t.Fatalf("GET %s 失败: %v", path, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s 返回 HTTP %d: %s", path, resp.StatusCode, string(body))
	}

	var envelope struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("解析标准响应失败: %v, body=%s", err, string(body))
	}
	if envelope.Code != 0 {
		t.Fatalf("标准响应 code 应为 0，实际为 %d: %s", envelope.Code, string(body))
	}

	var data T
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		t.Fatalf("解析 data 字段失败: %v, data=%s", err, string(envelope.Data))
	}
	return data
}

func assertOnlineExperienceGroupJSONContract(t *testing.T, token string) {
	t.Helper()

	resp, err := doRequest(t, http.MethodGet, "/api/v1/online-experience/groups", nil, token)
	if err != nil {
		t.Fatalf("GET /api/v1/online-experience/groups 失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/online-experience/groups 返回 HTTP %d: %s", resp.StatusCode, string(body))
	}

	var envelope struct {
		Code int                          `json:"code"`
		Data []map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("解析在线体验分组响应失败: %v, body=%s", err, string(body))
	}
	if envelope.Code != 0 {
		t.Fatalf("在线体验分组响应 code 应为 0，实际为 %d: %s", envelope.Code, string(body))
	}
	if len(envelope.Data) == 0 {
		return
	}

	first := envelope.Data[0]
	for _, key := range []string{"id", "name", "platform", "status"} {
		if _, ok := first[key]; !ok {
			t.Fatalf("在线体验分组响应缺少前端契约字段 %q，实际 keys=%v", key, mapKeys(first))
		}
	}
	for _, legacyKey := range []string{"ID", "Name", "Platform", "Status"} {
		if _, ok := first[legacyKey]; ok {
			t.Fatalf("在线体验分组响应不应暴露 Go 结构体字段 %q，实际 keys=%v", legacyKey, mapKeys(first))
		}
	}
}

func selectOnlineExperienceGroup(t *testing.T, groups []onlineExperienceGroup) onlineExperienceGroup {
	t.Helper()

	if raw := strings.TrimSpace(os.Getenv("ONLINE_EXPERIENCE_GROUP_ID")); raw != "" {
		groupID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || groupID <= 0 {
			t.Fatalf("ONLINE_EXPERIENCE_GROUP_ID 必须是正整数，实际为 %q", raw)
		}
		for _, group := range groups {
			if group.ID == groupID {
				return group
			}
		}
		t.Fatalf("ONLINE_EXPERIENCE_GROUP_ID=%d 不在当前用户可用在线体验分组中: %+v", groupID, groups)
	}

	for _, group := range groups {
		if strings.EqualFold(group.Platform, "openai") {
			return group
		}
	}
	t.Fatalf("在线体验分组列表中没有 OpenAI 分组: %+v", groups)
	return onlineExperienceGroup{}
}

func firstOnlineExperienceModel(models []onlineExperienceModel, modelType string) string {
	for _, model := range models {
		if strings.EqualFold(model.Type, modelType) && strings.TrimSpace(model.ID) != "" {
			return model.ID
		}
	}
	return ""
}

func mapKeys[V any](items map[string]V) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	return keys
}

func assertOnlineExperienceGatewayResponse(t *testing.T, operation string, resp *http.Response) []byte {
	t.Helper()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		if len(bytes.TrimSpace(body)) == 0 {
			t.Fatalf("%s 返回 200 但响应体为空", operation)
		}
		return body
	}

	requireUpstream := strings.EqualFold(os.Getenv("ONLINE_EXPERIENCE_REQUIRE_UPSTREAM"), "true")
	if requireUpstream {
		t.Fatalf("%s 要求真实上游成功，实际 HTTP %d: %s", operation, resp.StatusCode, string(body))
	}

	switch resp.StatusCode {
	case http.StatusForbidden, http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable:
		t.Logf("%s 已进入共享鉴权/网关上下文，但当前环境返回 HTTP %d: %s", operation, resp.StatusCode, summarizeOnlineExperienceBody(body))
		return body
	default:
		t.Fatalf("%s 返回非预期 HTTP %d: %s", operation, resp.StatusCode, string(body))
		return body
	}
}

func assertOnlineExperienceImageData(t *testing.T, body []byte) {
	t.Helper()

	var payload struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("解析图片响应失败: %v, body=%s", err, string(body))
	}
	if len(payload.Data) == 0 {
		t.Fatalf("图片响应缺少 data 数组: %s", string(body))
	}
}

func buildOnlineExperienceImageEditBody(t *testing.T, groupID int64, model string) ([]byte, string) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	mustWriteOnlineExperienceField(t, writer, "group_id", strconv.FormatInt(groupID, 10))
	mustWriteOnlineExperienceField(t, writer, "model", model)
	mustWriteOnlineExperienceField(t, writer, "prompt", "Change the background to a clean studio setting.")
	mustWriteOnlineExperienceField(t, writer, "size", "1024x1024")

	part, err := writer.CreateFormFile("image", "source.png")
	if err != nil {
		t.Fatalf("创建 multipart 图片字段失败: %v", err)
	}
	if _, err := part.Write(onePixelPNG(t)); err != nil {
		t.Fatalf("写入测试图片失败: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("关闭 multipart writer 失败: %v", err)
	}

	return body.Bytes(), writer.FormDataContentType()
}

func mustWriteOnlineExperienceField(t *testing.T, writer *multipart.Writer, name, value string) {
	t.Helper()
	if err := writer.WriteField(name, value); err != nil {
		t.Fatalf("写入 multipart 字段 %s 失败: %v", name, err)
	}
}

func doOnlineExperienceMultipartRequest(t *testing.T, path string, body []byte, contentType string, token string) (*http.Response, error) {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建 multipart 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 180 * time.Second}
	return client.Do(req)
}

func doOnlineExperienceJSONRequest(t *testing.T, path string, body []byte, token string) (*http.Response, error) {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建 JSON 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 180 * time.Second}
	return client.Do(req)
}

func onePixelPNG(t *testing.T) []byte {
	t.Helper()

	const encoded = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("解码测试 PNG 失败: %v", err)
	}
	return data
}

func summarizeOnlineExperienceBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) <= 300 {
		return text
	}
	return text[:300] + "..."
}
