package testing

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
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

	// CheckHeader is used to echo specified headers back
	CheckHeader = "check-header"
)

var (
	testServerAddr = ""
	testGrpcServer *grpc.Server

	testServerTLSAddr = ""
	testGrpcTLSServer *grpc.Server

	testServerMTLSAddr = ""
	testGrpcMTLSServer *grpc.Server

	testServerNoReflectAddr = ""
	testGrpcNoReflectServer *grpc.Server
)

type testService struct{}

func (testService) EmptyCall(ctx context.Context, req *grpc_testing.Empty) (*grpc_testing.Empty, error) {
	return req, nil
}

func (testService) UnaryCall(ctx context.Context, req *grpc_testing.SimpleRequest) (*grpc_testing.SimpleResponse, error) {
	checkHeaders := extractCheckHeaders(ctx)
	if len(checkHeaders) > 0 {
		imd, _ := metadata.FromIncomingContext(ctx)
		for _, hkv := range checkHeaders {
			values := imd.Get(hkv.key)
			if len(values) > 0 {
				if values[0] != hkv.value {
					return nil, status.Errorf(codes.InvalidArgument, "header '%s' validation failed", hkv.key)
				}
			}
		}
	}

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
	exitCode := extractStatusCodes(str.Context())
	if exitCode != codes.OK {
		return status.Error(exitCode, "error")
	}

	for {
		req, err := str.Recv()
		if err == io.EOF {
			return nil
		}

		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}

		if req.ResponseStatus != nil && req.ResponseStatus.Code != int32(codes.OK) {
			return status.Error(codes.Code(req.ResponseStatus.Code), "error")

		}

		resp := &grpc_testing.StreamingOutputCallResponse{Payload: &grpc_testing.Payload{}}
		for _, param := range req.ResponseParameters {
			if str.Context().Err() != nil {
				return str.Context().Err()
			}

			respSize := len(req.GetPayload().GetBody()) * int(param.GetSize())
			buf := make([]byte, 0, respSize)
			for i := 0; i < int(param.GetSize()); i++ {
				buf = append(buf, req.GetPayload().GetBody()...)
			}

			resp.Payload.Type = req.ResponseType
			resp.Payload.Body = buf

			if err := str.Send(resp); err != nil {
				return err
			}
		}
	}
}

func (testService) HalfDuplexCall(str grpc_testing.TestService_HalfDuplexCallServer) error {
	requests := []*grpc_testing.StreamingOutputCallRequest{}
	for {
		req, err := str.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}

		requests = append(requests, req)
	}

	for _, req := range requests {
		resp := &grpc_testing.StreamingOutputCallResponse{Payload: &grpc_testing.Payload{}}
		for _, param := range req.ResponseParameters {
			if str.Context().Err() != nil {
				return str.Context().Err()
			}

			respSize := len(req.GetPayload().GetBody()) * int(param.GetSize())
			buf := make([]byte, 0, respSize)
			for i := 0; i < int(param.GetSize()); i++ {
				buf = append(buf, req.GetPayload().GetBody()...)
			}

			resp.Payload.Type = req.ResponseType
			resp.Payload.Body = buf

			if err := str.Send(resp); err != nil {
				return err
			}
		}
	}

	return nil
}

func SetupTestServer() error {
	var err error
	testGrpcServer, testServerAddr, err = setupTestServer()
	if err != nil {
		return nil
	}

	testGrpcNoReflectServer, testServerNoReflectAddr, err = setupTestServerNoRelect()
	if err != nil {
		return nil
	}

	// no mTLS
	creds, err := getCreds(false)
	if err != nil {
		return err
	}

	testGrpcTLSServer, testServerTLSAddr, err = setupTestServer(creds)

	// mTLS
	mTLSCreds, err := getCreds(true)
	if err != nil {
		return err
	}

	testGrpcMTLSServer, testServerMTLSAddr, err = setupTestServer(mTLSCreds)

	return nil
}

func setupTestServer(opts ...grpc.ServerOption) (*grpc.Server, string, error) {
	server := createServer(opts...)
	reflection.Register(server)
	return createListener(server)
}

func setupTestServerNoRelect(opts ...grpc.ServerOption) (*grpc.Server, string, error) {
	server := createServer(opts...)
	return createListener(server)
}

func createServer(opts ...grpc.ServerOption) *grpc.Server {
	server := grpc.NewServer(opts...)
	testSvc := &testService{}
	grpc_testing.RegisterTestServiceService(server, grpc_testing.NewTestServiceService(testSvc))
	healthpb.RegisterHealthServer(server, &healthService{})
	return server
}

func createListener(server *grpc.Server) (*grpc.Server, string, error) {
	port := 0
	if l, err := net.Listen("tcp", "127.0.0.1:0"); err != nil {
		return nil, "", err
	} else {
		port = l.Addr().(*net.TCPAddr).Port
		go server.Serve(l)
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)

	return server, addr, nil
}

func StopTestServer() {
	stopTestServer(testGrpcServer)
	stopTestServer(testGrpcTLSServer)
	stopTestServer(testGrpcMTLSServer)
	stopTestServer(testGrpcNoReflectServer)
}

func stopTestServer(s *grpc.Server) {
	if s == nil {
		return
	}

	timer := time.AfterFunc(time.Duration(15*time.Second), func() {
		s.Stop()
	})
	defer timer.Stop()
	s.GracefulStop()

}

func TestServerAddr() string {
	return testServerAddr
}

func TestServerTLSAddr() string {
	return testServerTLSAddr
}

func TestServerMTLSAddr() string {
	return testServerMTLSAddr
}

func TestServerNoReflectAddr() string {
	return testServerNoReflectAddr
}

func TestServerInstance() *grpc.Server {
	return testGrpcServer
}

type headerKV struct {
	key   string
	value string
}

func extractCheckHeaders(ctx context.Context) []headerKV {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil
	}

	headers := md.Get(CheckHeader)
	res := make([]headerKV, len(headers))
	for i, h := range headers {
		hkv := headerKV{}
		kv := strings.Split(h, "=")
		if len(kv) > 0 {
			hkv.key = kv[0]
		}
		if len(kv) > 1 {
			hkv.value = kv[1]
		}
		res[i] = hkv
	}
	return res
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

func getCreds(mTLS bool) (grpc.ServerOption, error) {
	certificate, err := tls.LoadX509KeyPair(
		"../../testdata/certs/test_server.crt",
		"../../testdata/certs/test_server.key",
	)

	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	bs, err := ioutil.ReadFile("../../testdata/certs/test_ca.crt")
	if err != nil {
		log.Fatalf("failed to read client ca cert: %s", err)
	}

	ok := certPool.AppendCertsFromPEM(bs)
	if !ok {
		log.Fatal("failed to append client certs")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		ClientCAs:    certPool,
	}

	if mTLS {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	serverOption := grpc.Creds(credentials.NewTLS(tlsConfig))

	return serverOption, nil
}
