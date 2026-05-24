package service

import (
	"context"
	"errors"
	"fmt"
)

type apiKeyByUserAndNameFinder interface {
	GetByUserAndName(ctx context.Context, userID int64, name string) (*APIKey, error)
}

func (s *APIKeyService) EnsureOnlineExperienceAPIKey(ctx context.Context, userID, groupID int64) (*APIKey, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	group, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("get group: %w", err)
	}
	if group.Platform != PlatformOpenAI {
		return nil, ErrGroupNotAllowed
	}
	if !s.canUserBindGroup(ctx, user, group) {
		return nil, ErrGroupNotAllowed
	}

	internalName := BuildOnlineExperienceInternalAPIKeyName(groupID)

	repo, ok := s.apiKeyRepo.(apiKeyByUserAndNameFinder)
	if !ok {
		return nil, fmt.Errorf("api key repository does not support internal key lookup")
	}

	if existing, err := repo.GetByUserAndName(ctx, userID, internalName); err == nil {
		needsUpdate := existing.Status != StatusActive || existing.GroupID == nil || *existing.GroupID != groupID
		if needsUpdate {
			existing.Status = StatusActive
			existing.GroupID = &groupID
			if updateErr := s.apiKeyRepo.Update(ctx, existing); updateErr != nil {
				return nil, fmt.Errorf("update internal api key: %w", updateErr)
			}
		}
		existing.User = user
		existing.Group = group
		s.compileAPIKeyIPRules(existing)
		return existing, nil
	} else if !errors.Is(err, ErrAPIKeyNotFound) {
		return nil, fmt.Errorf("lookup internal api key: %w", err)
	}

	apiKey, err := s.Create(ctx, userID, CreateAPIKeyRequest{
		Name:    internalName,
		GroupID: &groupID,
	})
	if err != nil {
		return nil, err
	}

	apiKey.User = user
	apiKey.Group = group
	s.compileAPIKeyIPRules(apiKey)
	return apiKey, nil
}
