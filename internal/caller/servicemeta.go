package caller

import (
	"context"
	"fmt"
	"time"

	"github.com/jhump/protoreflect/v2/grpcreflect"
	"github.com/jhump/protoreflect/v2/protoresolve"
	"github.com/vadimi/grpc-client-cli/internal/rpc"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protoreflect"
	rpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

type serviceMetaData struct {
	connFact       *rpc.GrpcConnFactory
	target         string
	deadline       int
	protoImports   []string
	reflectVersion GrpcReflectVersion

	serviceMetaBase
}

type ServiceMetaDataConfig struct {
	ConnFact       *rpc.GrpcConnFactory
	Target         string
	ProtoImports   []string
	Deadline       int
	ReflectVersion GrpcReflectVersion
}

// NewServiceMetaData returns new instance of ServiceMetaData
// that reads service metadata by calling grpc Reflection service of the target
func NewServiceMetaData(cfg *ServiceMetaDataConfig) ServiceMetaData {
	return &serviceMetaData{
		connFact:       cfg.ConnFact,
		target:         cfg.Target,
		deadline:       cfg.Deadline,
		protoImports:   cfg.ProtoImports,
		reflectVersion: cfg.ReflectVersion,
	}
}

func (s *serviceMetaData) GetServiceMetaDataList(ctx context.Context) (ServiceMetaList, error) {
	conn, err := s.connFact.GetConn(s.target)
	if err != nil {
		return nil, err
	}
	callctx, cancel := context.WithTimeout(ctx, time.Duration(s.deadline)*time.Second)
	defer cancel()
	rc := s.grpcReflectClient(callctx, conn)

	services, err := rc.ListServices()
	if err != nil {
		defer rc.Reset()
		return nil, err
	}

	res := make([]*ServiceMeta, len(services))
	for i, svc := range services {
		svcDesc, err := resolveService(rc, svc)
		// sometimes ResolveService throws an error
		// when different proto files have different dependency protos named identically
		// For example service1.proto has common_types.proto and service2.proto has the same dependency
		// protoreflect library caches dependencies by name
		// so if we get an error, we can just recreate Client to reset internal cache and try again
		if err != nil {
			rc.Reset()
			// try only once here
			rc = s.grpcReflectClient(callctx, conn)
			svcDesc, err = resolveService(rc, svc)
			if err != nil {
				defer rc.Reset()
				return nil, err
			}
		}

		methods := make([]protoreflect.MethodDescriptor, svcDesc.Methods().Len())
		for j := range svcDesc.Methods().Len() {
			methods[j] = svcDesc.Methods().Get(j)
		}

		svcData := &ServiceMeta{
			File:    svcDesc.ParentFile(),
			Name:    string(svcDesc.FullName()),
			Methods: methods,
		}

		for _, m := range svcData.Methods {
			u := newJsonNamesUpdater()
			u.updateJSONNames(m.Input())
			u.updateJSONNames(m.Output())
		}
		res[i] = svcData
	}

	defer rc.Reset()
	return res, nil
}

func (s *serviceMetaData) GetAdditionalFiles() ([]protoreflect.FileDescriptor, error) {
	return s.serviceMetaBase.GetAdditionalFiles(s.protoImports)
}

func (s *serviceMetaData) grpcReflectClient(ctx context.Context, conn grpc.ClientConnInterface) *grpcreflect.Client {
	if s.reflectVersion == GrpcReflectAuto {
		return grpcreflect.NewClientAuto(ctx, conn)
	}
	rpbclient := rpb.NewServerReflectionClient(conn)
	return grpcreflect.NewClientV1Alpha(ctx, rpbclient)
}

// resolveService resolves a service descriptor by its full name,
// a replacement for protoreflect v1 grpcreflect.Client.ResolveService
func resolveService(rc *grpcreflect.Client, svc protoreflect.FullName) (protoreflect.ServiceDescriptor, error) {
	fd, err := rc.FileContainingSymbol(svc)
	if err != nil {
		return nil, err
	}

	dsc := protoresolve.FindDescriptorByNameInFile(fd, svc)
	if dsc == nil {
		return nil, fmt.Errorf("service %q not found", svc)
	}

	svcDesc, ok := dsc.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("%q is not a service", svc)
	}

	return svcDesc, nil
}
