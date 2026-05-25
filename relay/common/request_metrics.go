package common

import (
	"io"
	"sync"
	"time"
)

type requestMetricsState struct {
	mu sync.Mutex

	requestArrivedAt           time.Time
	requestBodySize            int64
	upstreamRequestStartedAt   time.Time
	upstreamResponseFinishedAt time.Time
	upstreamResponseBodySize   int64
}

// RequestMetricsSnapshot contains request lifecycle data safe to persist in logs.
type RequestMetricsSnapshot struct {
	RequestArrivedAt           time.Time
	RequestBodySize            int64
	UpstreamRequestStartedAt   time.Time
	UpstreamRequestBodySize    int64
	UpstreamResponseFinishedAt time.Time
	UpstreamResponseBodySize   int64
}

// SetRequestArrivalTime stores when the request first reached this server.
func (info *RelayInfo) SetRequestArrivalTime(t time.Time) {
	if info == nil || t.IsZero() {
		return
	}
	info.requestMetrics.mu.Lock()
	info.requestMetrics.requestArrivedAt = t
	info.requestMetrics.mu.Unlock()
}

// SetRequestBodySize stores the client request body size after middleware parsing.
func (info *RelayInfo) SetRequestBodySize(size int64) {
	if info == nil || size < 0 {
		return
	}
	info.requestMetrics.mu.Lock()
	info.requestMetrics.requestBodySize = size
	info.requestMetrics.mu.Unlock()
}

// SetUpstreamRequestBodySize stores the final upstream request body size.
func (info *RelayInfo) SetUpstreamRequestBodySize(size int64) {
	if info == nil || size < 0 {
		return
	}
	info.UpstreamRequestBodySize = size
}

// MarkUpstreamRequestStart stores when the outbound upstream request starts.
func (info *RelayInfo) MarkUpstreamRequestStart(t time.Time) {
	if info == nil || t.IsZero() {
		return
	}
	info.requestMetrics.mu.Lock()
	info.requestMetrics.upstreamRequestStartedAt = t
	info.requestMetrics.mu.Unlock()
}

// AddUpstreamResponseBodyBytes adds bytes read from the upstream response body.
func (info *RelayInfo) AddUpstreamResponseBodyBytes(n int64) {
	if info == nil || n <= 0 {
		return
	}
	info.requestMetrics.mu.Lock()
	info.requestMetrics.upstreamResponseBodySize += n
	info.requestMetrics.mu.Unlock()
}

// MarkUpstreamResponseFinished stores when the upstream response body is done.
func (info *RelayInfo) MarkUpstreamResponseFinished(t time.Time) {
	if info == nil || t.IsZero() {
		return
	}
	info.requestMetrics.mu.Lock()
	if info.requestMetrics.upstreamResponseFinishedAt.IsZero() {
		info.requestMetrics.upstreamResponseFinishedAt = t
	}
	info.requestMetrics.mu.Unlock()
}

// RequestMetricsSnapshot returns a consistent copy of lifecycle metrics.
func (info *RelayInfo) RequestMetricsSnapshot() RequestMetricsSnapshot {
	if info == nil {
		return RequestMetricsSnapshot{}
	}
	info.requestMetrics.mu.Lock()
	defer info.requestMetrics.mu.Unlock()
	return RequestMetricsSnapshot{
		RequestArrivedAt:           info.requestMetrics.requestArrivedAt,
		RequestBodySize:            info.requestMetrics.requestBodySize,
		UpstreamRequestStartedAt:   info.requestMetrics.upstreamRequestStartedAt,
		UpstreamRequestBodySize:    info.UpstreamRequestBodySize,
		UpstreamResponseFinishedAt: info.requestMetrics.upstreamResponseFinishedAt,
		UpstreamResponseBodySize:   info.requestMetrics.upstreamResponseBodySize,
	}
}

// TrackUpstreamResponseBody wraps a response body and records bytes and finish time.
func TrackUpstreamResponseBody(body io.ReadCloser, info *RelayInfo) io.ReadCloser {
	if body == nil || info == nil {
		return body
	}
	return &upstreamResponseBodyTracker{
		body: body,
		info: info,
	}
}

type upstreamResponseBodyTracker struct {
	body io.ReadCloser
	info *RelayInfo
}

func (t *upstreamResponseBodyTracker) Read(p []byte) (int, error) {
	n, err := t.body.Read(p)
	t.info.AddUpstreamResponseBodyBytes(int64(n))
	if err == io.EOF {
		t.info.MarkUpstreamResponseFinished(time.Now())
	}
	return n, err
}

func (t *upstreamResponseBodyTracker) Close() error {
	t.info.MarkUpstreamResponseFinished(time.Now())
	return t.body.Close()
}
