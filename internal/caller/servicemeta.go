package caller

import (
	"context"
	"time"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/vadimi/grpc-client-cli/internal/rpc"
	rpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

type ServiceMetaData interface {
	GetServiceMetaDataList() ([]*ServiceMeta, error)
}

type serviceMetaData struct {
	connFact *rpc.GrpcConnFactory
	target   string
	deadline int
}

type ServiceMeta struct {
	Name    string
	Methods []*desc.MethodDescriptor
	File    *desc.FileDescriptor
}

// NewServiceMetaData returns new instance of ServiceMetaData
// that reads service metadata by calling grpc Reflection service of the target
func NewServiceMetaData(connFact *rpc.GrpcConnFactory, target string, deadline int) ServiceMetaData {
	return &serviceMetaData{
		connFact: connFact,
		target:   target,
		deadline: deadline,
	}
}

func (s *serviceMetaData) GetServiceMetaDataList() ([]*ServiceMeta, error) {
	conn, err := s.connFact.GetConn(s.target)
	if err != nil {
		return nil, err
	}
	rpbclient := rpb.NewServerReflectionClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.deadline)*time.Second)
	defer cancel()
	rc := grpcreflect.NewClient(ctx, rpbclient)

	services, err := rc.ListServices()
	if err != nil {
		defer rc.Reset()
		return nil, err
	}

	res := make([]*ServiceMeta, len(services))
	for i, svc := range services {
		svcDesc, err := rc.ResolveService(svc)
		// sometimes ResolveService throws an error
		// when different proto files have different dependency protos named identically
		// For example service1.proto has common_types.proto and service2.proto has the same dependency
		// protoreflect library caches dependencies by name
		// so if we get an error, we can just recreate Client to reset internal cache and try again
		if err != nil {
			rc.Reset()
			// try only once here
			rc = grpcreflect.NewClient(ctx, rpbclient)
			svcDesc, err = rc.ResolveService(svc)
			if err != nil {
				defer rc.Reset()
				return nil, err
			}
		}

		svcData := &ServiceMeta{
			File:    svcDesc.GetFile(),
			Name:    svcDesc.GetFullyQualifiedName(),
			Methods: svcDesc.GetMethods(),
		}

		for _, m := range svcData.Methods {
			u := newJsonNamesUpdater()
			u.updateJSONNames(m.GetInputType())
			u.updateJSONNames(m.GetOutputType())
		}
		res[i] = svcData
	}

	defer rc.Reset()
	return res, nil
}
