package caller

import (
	"fmt"
	"io"

	"github.com/golang/protobuf/jsonpb"
	"github.com/pkg/errors"
	"github.com/vadimi/grpc-client-cli/internal/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"

	"context"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
)

// Proto message format
type MsgFormat int

func (f MsgFormat) String() string {
	switch f {
	case Text:
		return "text"
	case JSON:
		return "json"
	default:
		return "unknown"
	}
}

func ParseMsgFormat(s string) MsgFormat {
	if s == "text" {
		return Text
	}

	return JSON
}

const (
	JSON MsgFormat = iota
	Text
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

func (e *callerError) Unwrap() error {
	return e.Cause()
}

func newCallerError(err error) *callerError {
	return &callerError{err}
}

type ServiceCaller struct {
	connFact     *rpc.GrpcConnFactory
	inMsgFormat  MsgFormat
	outMsgFormat MsgFormat
}

func NewServiceCaller(connFact *rpc.GrpcConnFactory, inMsgFormat, outMsgFormat MsgFormat) *ServiceCaller {
	return &ServiceCaller{
		connFact:     connFact,
		inMsgFormat:  inMsgFormat,
		outMsgFormat: outMsgFormat,
	}
}

func (sc *ServiceCaller) CallStream(ctx context.Context, serviceTarget string, methodDesc *desc.MethodDescriptor, messages [][]byte, callOpts ...grpc.CallOption) (chan []byte, chan error) {
	errChan := make(chan error, 1)
	conn, err := sc.getConn(serviceTarget)
	if err != nil {
		errChan <- newCallerError(err)
		return nil, errChan
	}

	sd := grpc.StreamDesc{
		StreamName:    methodDesc.GetName(),
		ServerStreams: methodDesc.IsServerStreaming(),
		ClientStreams: methodDesc.IsClientStreaming(),
	}

	// fully qualified method name is needed here
	methodName := fmt.Sprintf("/%s/%s", methodDesc.GetService().GetFullyQualifiedName(), methodDesc.GetName())
	stream, err := conn.NewStream(ctx, &sd, methodName, callOpts...)
	if err != nil {
		errChan <- newCallerError(err)
		return nil, errChan
	}

	result := make(chan []byte)

	go func() {
		for {

			m := dynamic.NewMessage(methodDesc.GetOutputType())
			err := stream.RecvMsg(m)
			if err != nil {
				if err != io.EOF {
					errChan <- newCallerError(err)
				} else {
					close(errChan)
				}

				close(result)
				break
			}

			resMsg, err := sc.marshalMessage(m)
			if err != nil {
				errChan <- err
				close(result)
				break
			}
			result <- resMsg
		}
	}()

	for _, reqMsg := range messages {
		msg := dynamic.NewMessage(methodDesc.GetInputType())

		err := sc.unmarshalMessage(msg, reqMsg)
		if err != nil {
			errChan <- newCallerError(errors.Wrapf(err, "invalid input %s", sc.inMsgFormat.String()))
			return nil, errChan
		}

		err = stream.SendMsg(msg)
		// in case of EOF the real error should be discovered by stream.RecvMsg()
		if err == io.EOF {
			return nil, errChan
		}

		if err != nil {
			errChan <- newCallerError(err)
			return nil, errChan
		}

	}

	if err := stream.CloseSend(); err != nil {
		errChan <- newCallerError(err)
	}

	return result, errChan
}

// CallClientStream allows calling unary or client stream methods as they both return only a single result
func (sc *ServiceCaller) CallClientStream(ctx context.Context, serviceTarget string, methodDesc *desc.MethodDescriptor, messages [][]byte, callOpts ...grpc.CallOption) ([]byte, error) {
	if len(messages) == 0 {
		return nil, newCallerError(errors.New("empty requests are not allowed"))
	}

	resultCh, errChan := sc.CallStream(ctx, serviceTarget, methodDesc, messages, callOpts...)
	var result []byte
	for {
		select {
		case r := <-resultCh:
			if r != nil {
				result = r
			}
		case err := <-errChan:
			return result, err
		}
	}
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
	if sc.outMsgFormat == Text {
		return msg.MarshalText()
	}

	return msg.MarshalJSONPB(&jsonpb.Marshaler{
		EmitDefaults: true,
		Indent:       "  ",
		OrigName:     true,
	})
}

func (sc *ServiceCaller) unmarshalMessage(msg *dynamic.Message, b []byte) error {
	if sc.inMsgFormat == Text {
		return msg.UnmarshalText(b)
	}

	return msg.UnmarshalJSON(b)
}
