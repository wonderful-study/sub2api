package admin

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type fakeBatchAccountTester struct {
	results map[int64]*service.ScheduledTestResult
	errors  map[int64]error
}

func (f *fakeBatchAccountTester) RunTestBackground(ctx context.Context, accountID int64, modelID string) (*service.ScheduledTestResult, error) {
	if err := f.errors[accountID]; err != nil {
		return f.results[accountID], err
	}
	return f.results[accountID], nil
}

func TestRunBatchAccountTests_PartialSuccessRecoversSuccessfulAccounts(t *testing.T) {
	tester := &fakeBatchAccountTester{
		results: map[int64]*service.ScheduledTestResult{
			1: {Status: "success", LatencyMs: 11},
			2: {Status: "failed", LatencyMs: 22, ErrorMessage: "upstream failed"},
			3: {Status: "failed"},
		},
		errors: map[int64]error{
			3: errors.New("transport failed"),
		},
	}

	var mu sync.Mutex
	recovered := make([]int64, 0, 1)
	recovery := func(ctx context.Context, accountID int64) error {
		mu.Lock()
		defer mu.Unlock()
		recovered = append(recovered, accountID)
		return nil
	}

	out := runBatchAccountTests(context.Background(), []int64{1, 2, 3}, tester, recovery)

	require.Equal(t, 3, out.Total)
	require.Equal(t, 1, out.Success)
	require.Equal(t, 2, out.Failed)
	require.Len(t, out.Results, 3)
	require.Equal(t, int64(1), out.Results[0].AccountID)
	require.True(t, out.Results[0].Success)
	require.Equal(t, "success", out.Results[0].Status)
	require.Equal(t, int64(11), out.Results[0].LatencyMs)
	require.Equal(t, "upstream failed", out.Results[1].Error)
	require.Equal(t, "transport failed", out.Results[2].Error)

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []int64{1}, recovered)
}
