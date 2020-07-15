package rpc

import (
	"context"
	"sync/atomic"
	"time"

	"google.golang.org/grpc/stats"
)

type statsctxKey struct{}

type Stats struct {
	Duration time.Duration
	respSize int64
	reqSize  int64
}

func (s *Stats) RespSize() int64 {
	return atomic.LoadInt64(&s.respSize)
}

func (s *Stats) ReqSize() int64 {
	return atomic.LoadInt64(&s.reqSize)
}

// this method is based on
// https://github.com/cockroachdb/cockroach/blob/master/pkg/rpc/stats_handler.go
func (s *Stats) record(rpcStats stats.RPCStats) {
	switch v := rpcStats.(type) {
	case *stats.InHeader:
		atomic.AddInt64(&s.respSize, int64(v.WireLength))
	case *stats.InPayload:
		// TODO(spencer): remove the +5 offset on wire length here, which
		// is a temporary stand-in for the missing GRPC framing offset.
		// See: https://github.com/grpc/grpc-go/issues/1647.
		atomic.AddInt64(&s.respSize, int64(v.WireLength+5))
	case *stats.InTrailer:
		atomic.AddInt64(&s.respSize, int64(v.WireLength))
	case *stats.OutHeader:
		// No wire length.
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
