package testing

import (
	"context"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/interop/grpc_testing"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

var testServerAddr = ""
var testGrpcServer *grpc.Server

type testService struct{}

func (testService) EmptyCall(ctx context.Context, req *grpc_testing.Empty) (*grpc_testing.Empty, error) {
	return req, nil
}

func (testService) UnaryCall(ctx context.Context, req *grpc_testing.SimpleRequest) (*grpc_testing.SimpleResponse, error) {
	if req.ResponseStatus != nil && req.ResponseStatus.Code != int32(codes.OK) {
		return nil, status.Error(codes.Code(req.ResponseStatus.Code), "error")

	}

	return &grpc_testing.SimpleResponse{
		Payload: req.Payload,
	}, nil
}

func (testService) StreamingOutputCall(req *grpc_testing.StreamingOutputCallRequest, str grpc_testing.TestService_StreamingOutputCallServer) error {
	rsp := &grpc_testing.StreamingOutputCallResponse{Payload: &grpc_testing.Payload{}}
	for _, param := range req.ResponseParameters {
		if str.Context().Err() != nil {
			return str.Context().Err()
		}

		respSize := len(req.GetPayload().GetBody()) * int(param.GetSize())
		buf := make([]byte, 0, respSize)
		for i := 0; i < int(param.GetSize()); i++ {
			buf = append(buf, req.GetPayload().GetBody()...)
		}

		rsp.Payload.Type = req.ResponseType
		rsp.Payload.Body = buf

		if err := str.Send(rsp); err != nil {
			return err
		}
	}

	return nil
}

func (testService) StreamingInputCall(str grpc_testing.TestService_StreamingInputCallServer) error {
	return nil
}

func (testService) FullDuplexCall(str grpc_testing.TestService_FullDuplexCallServer) error {
	return nil
}

func (testService) HalfDuplexCall(str grpc_testing.TestService_HalfDuplexCallServer) error {
	return nil
}

func SetupTestServer() error {
	testGrpcServer = grpc.NewServer()
	testSvc := &testService{}
	grpc_testing.RegisterTestServiceServer(testGrpcServer, testSvc)
	reflection.Register(testGrpcServer)

	port := 0
	if l, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		return err
	} else {
		port = l.Addr().(*net.TCPAddr).Port
		go testGrpcServer.Serve(l)
	}

	testServerAddr = fmt.Sprintf("127.0.0.1:%d", port)

	return nil
}

func StopTestServer() {
	if testGrpcServer == nil {
		return
	}

	timer := time.AfterFunc(time.Duration(15*time.Second), func() {
		testGrpcServer.Stop()
	})
	defer timer.Stop()
	testGrpcServer.GracefulStop()

}

func TestServerAddr() string {
	return testServerAddr
}
