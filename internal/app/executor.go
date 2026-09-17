package app

import (
	"context"

	"github.com/vadimi/grpc-client-cli/internal/caller"
	"github.com/vadimi/grpc-client-cli/internal/rpc"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// CallExecutor performs grpc calls for a selected service method. The
// interactive application runs calls through this interface so they can be
// exercised with fake implementations in tests.
type CallExecutor interface {
	// CallUnary invokes a unary or client streaming method and returns its
	// single result.
	CallUnary(ctx context.Context, method protoreflect.MethodDescriptor, messages [][]byte) ([]byte, error)
	// CallStreaming invokes a server or bidi streaming method. Every result
	// is passed to onResult as soon as it arrives. It returns the terminal
	// status of the call after all results were delivered.
	CallStreaming(ctx context.Context, method protoreflect.MethodDescriptor, messages [][]byte, onResult func([]byte)) error
}

type callExecutor struct {
	connFact     *rpc.GrpcConnFactory
	target       string
	inFormat     caller.MsgFormat
	outFormat    caller.MsgFormat
	outJsonNames bool
}

// NewCallExecutor returns a CallExecutor that calls serviceTarget through
// connFact, marshaling requests with inFormat and results with outFormat.
func NewCallExecutor(connFact *rpc.GrpcConnFactory, target string, inFormat, outFormat caller.MsgFormat, outJsonNames bool) CallExecutor {
	return &callExecutor{
		connFact:     connFact,
		target:       target,
		inFormat:     inFormat,
		outFormat:    outFormat,
		outJsonNames: outJsonNames,
	}
}

func (e *callExecutor) serviceCaller() *caller.ServiceCaller {
	return caller.NewServiceCaller(e.connFact, e.inFormat, e.outFormat, e.outJsonNames)
}

func (e *callExecutor) CallUnary(ctx context.Context, method protoreflect.MethodDescriptor, messages [][]byte) ([]byte, error) {
	return e.serviceCaller().CallClientStream(ctx, e.target, method, messages, grpc.WaitForReady(true))
}

func (e *callExecutor) CallStreaming(ctx context.Context, method protoreflect.MethodDescriptor, messages [][]byte, onResult func([]byte)) error {
	result, errCh := e.serviceCaller().CallStream(ctx, e.target, method, messages, grpc.WaitForReady(true))
	for r := range result {
		if onResult != nil {
			onResult(r)
		}
	}

	return <-errCh
}
