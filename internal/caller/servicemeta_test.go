package caller

import (
	"context"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vadimi/grpc-client-cli/internal/rpc"
	"google.golang.org/grpc"
)

func TestGrpcReflectVersions(t *testing.T) {
	const reflectV1alphaMethod = "/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo"
	const reflectV1Method = "/grpc.reflection.v1.ServerReflection/ServerReflectionInfo"

	tests := []struct {
		name           string
		expectedMethod string
		version        GrpcReflectVersion
	}{
		{name: "v1alpha", version: GrpcReflectV1Alpha, expectedMethod: reflectV1alphaMethod},
		{name: "auto", version: GrpcReflectAuto, expectedMethod: reflectV1Method},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lis, err := net.Listen("tcp", "localhost:0")
			if err != nil {
				t.Fatalf("failed to listen: %v", err)
			}

			unknownHandler := func(_ any, stream grpc.ServerStream) error {
				m, ok := grpc.Method(stream.Context())
				assert.True(t, ok)
				assert.Equal(t, tt.expectedMethod, m, "wrong grpc reflect method called")
				return nil
			}

			s := grpc.NewServer(grpc.UnknownServiceHandler(unknownHandler))
			defer s.Stop()
			go s.Serve(lis)

			svc := NewServiceMetaData(&ServiceMetaDataConfig{
				ConnFact:       rpc.NewGrpcConnFactory(),
				Target:         lis.Addr().String(),
				ReflectVersion: tt.version,
				Deadline:       15,
			})

			_, err = svc.GetServiceMetaDataList(context.Background())
			assert.ErrorIs(t, err, io.EOF)
		})
	}
}
