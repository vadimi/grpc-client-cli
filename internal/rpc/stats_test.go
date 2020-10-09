package rpc

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/stats"
)

func TestRPCStatsRecording(t *testing.T) {
	ctx := WithStatsCtx(context.Background())
	h := newStatsHanler()

	begin := time.Now()
	end := begin.Add(1 * time.Second)

	h.HandleRPC(ctx, &stats.InHeader{
		WireLength: 2,
	})

	h.HandleRPC(ctx, &stats.InPayload{
		WireLength: 2,
	})

	h.HandleRPC(ctx, &stats.InTrailer{
		WireLength: 2,
	})

	h.HandleRPC(ctx, &stats.OutPayload{
		WireLength: 2,
	})

	h.HandleRPC(ctx, &stats.OutTrailer{
		WireLength: 2,
	})

	h.HandleRPC(ctx, &stats.End{
		BeginTime: begin,
		EndTime:   end,
	})

	s := ExtractRpcStats(ctx)

	expDur := end.Sub(begin)
	if expDur != s.Duration {
		t.Errorf("invalid stats duration: %v, expected %v", s.Duration, expDur)
	}

	expectedReqSize := int64(4)
	if s.ReqSize() != expectedReqSize {
		t.Errorf("invalid req size: %d, expected: %d", s.ReqSize(), expectedReqSize)
	}

	expectedRespSize := int64(6)
	if s.RespSize() != expectedRespSize {
		t.Errorf("invalid resp size: %d, expected: %d", s.RespSize(), expectedRespSize)
	}
}
