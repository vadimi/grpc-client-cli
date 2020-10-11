package rpc

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/stats"
)

type statsctxKey struct{}

type Stats struct {
	Duration     time.Duration
	respSize     int64
	reqSize      int64
	reqHeaders   metadata.MD
	respHeaders  metadata.MD
	respTrailers metadata.MD
	fullMethod   string
	sync.RWMutex
}

func (s *Stats) RespSize() int64 {
	return atomic.LoadInt64(&s.respSize)
}

func (s *Stats) ReqSize() int64 {
	return atomic.LoadInt64(&s.reqSize)
}

func (s *Stats) ReqHeaders() metadata.MD {
	s.RLock()
	defer s.RUnlock()
	return s.reqHeaders
}

func (s *Stats) RespHeaders() metadata.MD {
	s.RLock()
	defer s.RUnlock()
	return s.respHeaders
}

func (s *Stats) RespTrailers() metadata.MD {
	s.RLock()
	defer s.RUnlock()
	return s.respTrailers
}

func (s *Stats) FullMethod() string {
	s.RLock()
	defer s.RUnlock()
	return s.fullMethod
}

// this method is based on
// https://github.com/cockroachdb/cockroach/blob/master/pkg/rpc/stats_handler.go
func (s *Stats) record(rpcStats stats.RPCStats) {
	switch v := rpcStats.(type) {
	case *stats.InHeader:
		s.Lock()
		atomic.AddInt64(&s.respSize, int64(v.WireLength))
		s.respHeaders = v.Header.Copy()
		s.Unlock()
	case *stats.InPayload:
		atomic.AddInt64(&s.respSize, int64(v.WireLength))
	case *stats.InTrailer:
		atomic.AddInt64(&s.respSize, int64(v.WireLength))
		s.Lock()
		s.respTrailers = v.Trailer.Copy()
		s.Unlock()
	case *stats.OutHeader:
		// No wire length.
		s.Lock()
		s.reqHeaders = v.Header.Copy()
		s.fullMethod = v.FullMethod
		s.Unlock()
	case *stats.OutPayload:
		atomic.AddInt64(&s.reqSize, int64(v.WireLength))
	case *stats.OutTrailer:
		atomic.AddInt64(&s.reqSize, int64(v.WireLength))
	case *stats.End:
		s.Duration = v.EndTime.Sub(v.BeginTime)
	}
}

type statsHandler struct{}

func newStatsHanler() stats.Handler {
	return &statsHandler{}
}

func (cs *statsHandler) TagRPC(ctx context.Context, rti *stats.RPCTagInfo) context.Context {
	return ctx
}

func (cs *statsHandler) HandleRPC(ctx context.Context, rpcStats stats.RPCStats) {
	stats := ExtractRpcStats(ctx)
	if stats != nil {
		stats.record(rpcStats)
	}
}

func (cs *statsHandler) TagConn(ctx context.Context, cti *stats.ConnTagInfo) context.Context {
	return ctx
}

func (cs *statsHandler) HandleConn(context.Context, stats.ConnStats) {
}

func WithStatsCtx(parentCtx context.Context) context.Context {
	stats := ExtractRpcStats(parentCtx)
	if stats == nil {
		stats = &Stats{}
		return context.WithValue(parentCtx, statsctxKey{}, stats)
	}
	return parentCtx
}

func ExtractRpcStats(ctx context.Context) *Stats {
	val := ctx.Value(statsctxKey{})
	if val == nil {
		return nil
	}

	stats, ok := val.(*Stats)
	if !ok {
		return nil
	}

	return stats
}
