package allino

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"
)

type JobStore interface {
	List(ctx context.Context, filter JobListFilter) ([]JobInfo, error)
	Total(ctx context.Context, rootID ...string) (map[string]int, error)
	Result(ctx context.Context, key string, volatile bool) (JobResult, error)
	Requeue(ctx context.Context, key string, delaySec int) error
	Free(ctx context.Context, key string) error
	Status() JobStoreStatus
}

type JobListFilter struct {
	Statuses []string
	Limit    int
	Offset   int
}

type JobResult struct {
	Info   JobInfo `json:"info"`
	Output []byte  `json:"output,omitempty"`
	Error  []byte  `json:"error,omitempty"`
}

type JobStoreStatus struct {
	Configured      bool     `json:"configured"`
	Backend         string   `json:"backend,omitempty"`
	Handlers        []string `json:"handlers,omitempty"`
	LockedHandlers  []string `json:"lockedHandlers,omitempty"`
	ActiveJobs      int64    `json:"activeJobs"`
	Concurrency     int      `json:"concurrency"`
	DequeueAttempts int64    `json:"dequeueAttempts"`
}

type sqlJobStore struct {
	server   *Server
	strategy *callSQLStrategy
}

func (s *Server) JobStore() JobStore {
	if s == nil || s.callSQLStrategy == nil {
		return nil
	}
	return &sqlJobStore{
		server:   s,
		strategy: s.callSQLStrategy,
	}
}

func (s *sqlJobStore) List(ctx context.Context, filter JobListFilter) ([]JobInfo, error) {
	statuses, err := jobStatusCodes(filter.Statuses)
	if err != nil {
		return nil, err
	}
	return s.strategy.List(ctx, statuses, filter.Limit, filter.Offset)
}

func (s *sqlJobStore) Total(ctx context.Context, rootID ...string) (map[string]int, error) {
	raw, err := s.strategy.Total(ctx, rootID...)
	if err != nil {
		return nil, err
	}
	out := make(map[string]int, len(raw))
	for status, count := range raw {
		out[jobStatusName(status)] += count
	}
	return out, nil
}

func (s *sqlJobStore) Result(ctx context.Context, key string, volatile bool) (JobResult, error) {
	info, output, errJSON, err := s.strategy.Result(ctx, key, volatile)
	if err != nil {
		return JobResult{}, err
	}
	return JobResult{
		Info:   info,
		Output: output,
		Error:  errJSON,
	}, nil
}

func (s *sqlJobStore) Requeue(ctx context.Context, key string, delaySec int) error {
	return s.strategy.requeue(ctx, key, delaySec)
}

func (s *sqlJobStore) Free(ctx context.Context, key string) error {
	return s.strategy.Free(ctx, key)
}

func (s *sqlJobStore) Status() JobStoreStatus {
	status := JobStoreStatus{
		Configured:  true,
		Backend:     s.strategy.name,
		Concurrency: s.server.Config.JobConfig.Concurrency,
	}
	if s.server.jobManager == nil {
		return status
	}
	status.Handlers = s.server.jobManager.handlers.Slice()
	status.LockedHandlers = s.server.jobManager.lockedHandlers.Slice()
	status.ActiveJobs = atomic.LoadInt64(&s.server.jobManager.activeJobs)
	status.DequeueAttempts = atomic.LoadInt64(&s.server.jobManager.attempt)
	return status
}

func jobStatusCodes(statuses []string) ([]int, error) {
	if len(statuses) == 0 {
		return nil, nil
	}
	out := make([]int, 0, len(statuses))
	for _, status := range statuses {
		code, err := jobStatusCode(status)
		if err != nil {
			return nil, err
		}
		out = append(out, code)
	}
	return out, nil
}

func jobStatusCode(status string) (int, error) {
	switch status {
	case "queued":
		return statusQueued, nil
	case "leased":
		return statusLeased, nil
	case "done":
		return statusDone, nil
	case "error":
		return statusError, nil
	}
	code, err := strconv.Atoi(status)
	if err != nil || code < 0 || code >= len(sqlStatusCodeStrings) {
		return 0, fmt.Errorf("invalid job status: %s", status)
	}
	return code, nil
}

func jobStatusName(status string) string {
	code, err := strconv.Atoi(status)
	if err == nil && code >= 0 && code < len(sqlStatusCodeStrings) {
		return sqlStatusCodeStrings[code]
	}
	return status
}
