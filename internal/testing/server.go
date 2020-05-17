package testing

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/interop/grpc_testing"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const (
	// grpc code to exit the method
	// useful when testing errors behavior
	MethodExitCode = "exit-code"
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
	if req.ResponseStatus != nil && req.ResponseStatus.Code != int32(codes.OK) {
		return status.Error(codes.Code(req.ResponseStatus.Code), "error")

	}

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
	exitCode := extractStatusCodes(str.Context())
	if exitCode != codes.OK {
		return status.Error(exitCode, "error")
	}

	size := 0
	for {
		req, err := str.Recv()
		if err == io.EOF {
			return str.SendAndClose(&grpc_testing.StreamingInputCallResponse{
				AggregatedPayloadSize: int32(size),
			})
		}

		size += len(req.Payload.Body)

		if err != nil {
			return err
		}
	}
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
	healthpb.RegisterHealthServer(testGrpcServer, &healthService{})
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

func TestServerInstance() *grpc.Server {
	return testGrpcServer
}

func extractStatusCodes(ctx context.Context) codes.Code {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return codes.OK
	}

	values := md.Get(MethodExitCode)
	if len(values) == 0 {
		return codes.OK
	}

	i, err := strconv.Atoi(values[len(values)-1])
	if err != nil {
		return codes.OK
	}
	return codes.Code(i)
}
