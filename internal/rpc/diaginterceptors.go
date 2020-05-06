package rpc

import (
	"context"
	"time"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
)

type DiagInfo struct {
	Duration time.Duration
	RespSize int
	ReqSize  int
}

type diagctxKey struct{}

func DiagContext() context.Context {
	return context.WithValue(context.Background(), diagctxKey{}, &DiagInfo{})
}

func ExtractDiagInfo(ctx context.Context) *DiagInfo {
	val := ctx.Value(diagctxKey{})
	if val == nil {
		return nil
	}

	diagInfo, ok := val.(*DiagInfo)
	if !ok {
		return nil
	}

	return diagInfo
}

func DiagUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		diagInfo := ExtractDiagInfo(ctx)
		if diagInfo == nil {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		if m, ok := req.(proto.Message); ok {
			diagInfo.ReqSize = proto.Size(m)
		}
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		diagInfo.Duration = time.Since(start)
		if m, ok := reply.(proto.Message); ok {
			diagInfo.RespSize = proto.Size(m)
		}
		return err
	}
}

func DiagStreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		diagInfo := ExtractDiagInfo(ctx)
		if diagInfo == nil {
			return streamer(ctx, desc, cc, method, opts...)
		}
		clientStream, err := streamer(ctx, desc, cc, method, opts...)
		newStream := &diagClientStream{ClientStream: clientStream, diagInfo: diagInfo}
		return newStream, err
	}
}

type diagClientStream struct {
	grpc.ClientStream
	diagInfo *DiagInfo
}

func (l *diagClientStream) SendMsg(m interface{}) error {
	if pm, ok := m.(proto.Message); ok {
		l.diagInfo.ReqSize += proto.Size(pm)
	}
	start := time.Now()
	defer func() {
		l.diagInfo.Duration += time.Since(start)
	}()
	return l.ClientStream.SendMsg(m)
}

func (l *diagClientStream) RecvMsg(m interface{}) error {
	start := time.Now()
	err := l.ClientStream.RecvMsg(m)
	if err == nil {
		if pm, ok := m.(proto.Message); ok {
			l.diagInfo.RespSize += proto.Size(pm)
		}
	}
	d := time.Since(start)
	l.diagInfo.Duration += d
	return err
}
