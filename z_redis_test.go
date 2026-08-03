package allino_test

import (
	"context"
	"testing"
	"time"
)

func requireRedis(t *testing.T) {
	t.Helper()

	if s.Redis == nil {
		t.Skip("Redis not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if err := s.Redis.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}
}
