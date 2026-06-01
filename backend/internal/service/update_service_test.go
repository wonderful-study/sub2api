//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type updateServiceCacheStub struct {
	data string
}

func (s *updateServiceCacheStub) GetUpdateInfo(context.Context) (string, error) {
	if s.data == "" {
		return "", errors.New("cache miss")
	}
	return s.data, nil
}

func (s *updateServiceCacheStub) SetUpdateInfo(_ context.Context, data string, _ time.Duration) error {
	s.data = data
	return nil
}

type updateServiceGitHubClientStub struct {
	release *GitHubRelease
}

func (s *updateServiceGitHubClientStub) FetchLatestRelease(context.Context, string) (*GitHubRelease, error) {
	return s.release, nil
}

func (s *updateServiceGitHubClientStub) FetchReleaseByTag(context.Context, string, string) (*GitHubRelease, error) {
	panic("FetchReleaseByTag should not be called when no update is available")
}

func (s *updateServiceGitHubClientStub) DownloadFile(context.Context, string, string, int64) error {
	panic("DownloadFile should not be called when no update is available")
}

func (s *updateServiceGitHubClientStub) FetchChecksumFile(context.Context, string) ([]byte, error) {
	panic("FetchChecksumFile should not be called when no update is available")
}

func (s *updateServiceGitHubClientStub) DispatchWorkflow(context.Context, string, string, string, string, map[string]string) (string, error) {
	panic("DispatchWorkflow should not be called when no update is available")
}

func TestUpdateServicePerformUpdateNoUpdateReturnsSentinel(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{
			release: &GitHubRelease{
				TagName: "v0.1.132",
				Name:    "v0.1.132",
			},
		},
		"0.1.132",
		"release",
		UpdateOptions{},
	)

	_, err := svc.PerformUpdate(context.Background())

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoUpdateAvailable))
	require.ErrorIs(t, err, ErrNoUpdateAvailable)
}

func TestCompareVersionsNormalizesForkVersions(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    int
	}{
		{
			name:    "fork suffix equals upstream patch",
			current: "0.1.133-fork.1",
			latest:  "0.1.133",
			want:    0,
		},
		{
			name:    "custom suffix equals upstream patch",
			current: "0.1.133-xxx",
			latest:  "0.1.133",
			want:    0,
		},
		{
			name:    "fourth numeric segment equals upstream patch",
			current: "0.1.133.1",
			latest:  "0.1.133",
			want:    0,
		},
		{
			name:    "fork suffix is lower than next upstream patch",
			current: "0.1.133-fork.1",
			latest:  "0.1.134",
			want:    -1,
		},
		{
			name:    "newer fork suffix is greater than older upstream patch",
			current: "0.1.134-fork.1",
			latest:  "0.1.133",
			want:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, compareVersions(tt.current, tt.latest))
		})
	}
}
