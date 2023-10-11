package caller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/vadimi/grpc-client-cli/internal/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/jhump/protoreflect/desc"
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

type GrpcReflectVersion int

func ParseGrpcReflectVersion(s string) GrpcReflectVersion {
	switch strings.ToLower(s) {
	case "v1alpha":
		return GrpcReflectV1Alpha
	case "auto":
		return GrpcReflectAuto
	default:
		return GrpcReflectV1Alpha
	}
}

const (
	JSON MsgFormat = iota
	Text
)

const (
	GrpcReflectV1Alpha GrpcReflectVersion = iota
	// automatically determine which grpc reflection version to use
	GrpcReflectAuto
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
	fdescCache   *FileDescCache
	outJsonNames bool
}

func NewServiceCaller(connFact *rpc.GrpcConnFactory, inMsgFormat, outMsgFormat MsgFormat, fdescCache *FileDescCache, outJsonNames bool) *ServiceCaller {
	return &ServiceCaller{
		connFact:     connFact,
		inMsgFormat:  inMsgFormat,
		outMsgFormat: outMsgFormat,
		fdescCache:   fdescCache,
		outJsonNames: outJsonNames,
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
			m := dynamicpb.NewMessage(methodDesc.GetOutputType().UnwrapMessage())

			// m := dynamic.NewMessage(methodDesc.GetOutputType())
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
		// msg := dynamic.NewMessage(methodDesc.GetInputType())
		msg := dynamicpb.NewMessage(methodDesc.GetInputType().UnwrapMessage())

		err := sc.unmarshalMessage(msg, reqMsg)
		if err != nil {
			errChan <- newCallerError(fmt.Errorf("invalid input %s: %w", sc.inMsgFormat.String(), err))
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

func (sc *ServiceCaller) marshalMessage(msg *dynamicpb.Message) ([]byte, error) {
	if sc.outMsgFormat == Text {
		return prototext.Marshal(msg)
		// return msg.MarshalText()
	}

	return protojson.MarshalOptions{
		EmitUnpopulated: true,
		Indent:          "  ",
		UseProtoNames:   !sc.outJsonNames,
		Resolver:        NewRes(sc.fdescCache),
	}.Marshal(msg)

	// return msg.MarshalJSONPB(&jsonpb.Marshaler{
	// 	EmitDefaults: true,
	// 	Indent:       "  ",
	// 	OrigName:     !sc.outJsonNames,
	// 	AnyResolver:  &anyResolver{sc.fdescCache},
	// })
}

func (sc *ServiceCaller) unmarshalMessage(msg *dynamicpb.Message, b []byte) error {
	if sc.inMsgFormat == Text {
		return prototext.Unmarshal(b, msg)
		// return msg.UnmarshalText(b)
	}

	return protojson.UnmarshalOptions{
		AllowPartial: true,
		Resolver:     NewRes(sc.fdescCache),
	}.Unmarshal(b, msg)

	// return msg.UnmarshalJSONPB(&jsonpb.Unmarshaler{
	// 	AnyResolver: &anyResolver{sc.fdescCache},
	// }, b)
}
