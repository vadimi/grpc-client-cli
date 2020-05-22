package caller

import (
	"io"

	"github.com/golang/protobuf/jsonpb"
	"github.com/pkg/errors"
	"github.com/vadimi/grpc-client-cli/internal/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"

	"golang.org/x/net/context"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
)

type temporary interface {
	Temporary() bool
}

// IsErrTransient returns true if err is t.
func IsErrTransient(err error) bool {
	te, ok := err.(temporary)
	return ok && te.Temporary()
}

type callerError struct {
	err error
}

func (e *callerError) Error() string {
	return e.err.Error()
}

func (e *callerError) Temporary() bool {
	code := status.Code(e.err)
	return code != codes.Unavailable
}

func (e *callerError) Cause() error {
	return e.err
}

func newCallerError(err error) *callerError {
	return &callerError{err}
}

type ServiceCaller struct {
	connFact *rpc.GrpcConnFactory
}

func NewServiceCaller(connFact *rpc.GrpcConnFactory) *ServiceCaller {
	return &ServiceCaller{connFact}
}

func (sc *ServiceCaller) CallJSON(ctx context.Context, serviceTarget string, methodDesc *desc.MethodDescriptor, reqJSON []byte, callOpts ...grpc.CallOption) ([]byte, error) {
	msg := dynamic.NewMessage(methodDesc.GetInputType())

	err := msg.UnmarshalJSON(reqJSON)
	if err != nil {
		return nil, newCallerError(errors.Wrap(err, "invalid input json"))
	}

	conn, err := sc.getConn(serviceTarget)
	if err != nil {
		return nil, err
	}

	resp := dynamic.NewMessage(methodDesc.GetOutputType())
	stub := grpcdynamic.NewStub(conn)
	protoRes, err := stub.InvokeRpc(ctx, methodDesc, msg, callOpts...)
	if err != nil {
		return nil, newCallerError(err)
	}

	err = resp.ConvertFrom(protoRes)
	if err != nil {
		return nil, err
	}

	return sc.marshalMessage(resp)
}

func (sc *ServiceCaller) CallServerStream(ctx context.Context, serviceTarget string, methodDesc *desc.MethodDescriptor, reqJSON []byte, callOpts ...grpc.CallOption) (chan []byte, chan error) {
	msg := dynamic.NewMessage(methodDesc.GetInputType())
	errChan := make(chan error, 1)

	err := msg.UnmarshalJSON(reqJSON)
	if err != nil {
		errChan <- newCallerError(errors.Wrap(err, "invalid input json"))
		return nil, errChan
	}

	conn, err := sc.getConn(serviceTarget)
	if err != nil {
		return nil, errChan
	}

	result := make(chan []byte)
	resp := dynamic.NewMessage(methodDesc.GetOutputType())
	stub := grpcdynamic.NewStub(conn)
	stream, err := stub.InvokeRpcServerStream(ctx, methodDesc, msg, callOpts...)
	if err != nil {
		errChan <- newCallerError(err)
		return nil, errChan
	}

	go func() {
		for {
			m, err := stream.RecvMsg()
			if err != nil {
				if err != io.EOF {
					errChan <- newCallerError(err)
				} else {
					close(errChan)
				}

				close(result)
				break
			}

			err = resp.ConvertFrom(m)
			if err != nil {
				errChan <- err
				close(result)
				break
			}

			json, err := sc.marshalMessage(resp)
			if err != nil {
				errChan <- err
				close(result)
				break
			}
			result <- json
		}
	}()

	return result, errChan
}

func (sc *ServiceCaller) CallClientStream(ctx context.Context, serviceTarget string, methodDesc *desc.MethodDescriptor, reqJSON [][]byte, callOpts ...grpc.CallOption) ([]byte, error) {
	if len(reqJSON) == 0 {
		return nil, newCallerError(errors.New("empty requests are not allowed"))
	}

	conn, err := sc.getConn(serviceTarget)
	if err != nil {
		return nil, err
	}

	resp := dynamic.NewMessage(methodDesc.GetOutputType())
	stub := grpcdynamic.NewStub(conn)
	stream, err := stub.InvokeRpcClientStream(ctx, methodDesc, callOpts...)
	if err != nil {
		return nil, newCallerError(err)
	}

	for _, reqMsg := range reqJSON {
		msg := dynamic.NewMessage(methodDesc.GetInputType())

		err := msg.UnmarshalJSON(reqMsg)
		if err != nil {
			return nil, newCallerError(errors.Wrap(err, "invalid input json"))
		}

		err = stream.SendMsg(msg)
		if err != nil {
			return nil, newCallerError(err)
		}
	}

	protoRes, err := stream.CloseAndReceive()
	if err != nil {
		return nil, newCallerError(err)
	}

	err = resp.ConvertFrom(protoRes)
	if err != nil {
		return nil, err
	}

	return sc.marshalMessage(resp)
}

func (sc *ServiceCaller) getConn(target string) (*grpc.ClientConn, error) {
	conn, err := sc.connFact.GetConn(target)
	if err != nil {
		return nil, err
	}

	if conn.GetState() != connectivity.Ready {
		conn.ResetConnectBackoff()
	}

	return conn, err
}

func (sc *ServiceCaller) marshalMessage(msg *dynamic.Message) ([]byte, error) {
	return msg.MarshalJSONPB(&jsonpb.Marshaler{
		EmitDefaults: true,
		Indent:       "  ",
		OrigName:     true,
	})
}
