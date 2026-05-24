package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountFromService_PreservesSubscriptionExpiryWithoutAccountExpiry(t *testing.T) {
	account := &service.Account{
		ID:       42,
		Name:     "openai-account",
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeOAuth,
		Status:   service.StatusActive,
		Credentials: map[string]any{
			"subscription_expires_at": "2026-05-10T00:00:00Z",
		},
	}

	out := AccountFromService(account)

	require.NotNil(t, out)
	require.Nil(t, out.ExpiresAt)
	require.Equal(t, "2026-05-10T00:00:00Z", out.Credentials["subscription_expires_at"])
}
