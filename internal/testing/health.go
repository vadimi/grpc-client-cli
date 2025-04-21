package testing

import (
	"context"

	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type healthService struct {
	healthpb.UnimplementedHealthServer
}

func (s *healthService) Check(ctx context.Context, in *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	response := &healthpb.HealthCheckResponse{
		Status: healthpb.HealthCheckResponse_SERVING,
	}

	if in.Service == "error" {
		return nil, status.Error(codes.Code(codes.Internal), "error")
	}

	if in.Service == "unhealthy" {
		response.Status = healthpb.HealthCheckResponse_NOT_SERVING
	}

	return response, nil
}

func (s *healthService) Watch(in *healthpb.HealthCheckRequest, stream healthpb.Health_WatchServer) error {
	return status.Error(codes.Unimplemented, "Watching is not supported")
}
