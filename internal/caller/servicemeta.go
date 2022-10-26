package caller

import (
	"context"
	"time"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/vadimi/grpc-client-cli/internal/rpc"
)

type serviceMetaData struct {
	connFact     *rpc.GrpcConnFactory
	target       string
	deadline     int
	protoImports []string

	serviceMetaBase
}

// NewServiceMetaData returns new instance of ServiceMetaData
// that reads service metadata by calling grpc Reflection service of the target
func NewServiceMetaData(connFact *rpc.GrpcConnFactory, target string, deadline int, protoImports []string) ServiceMetaData {
	return &serviceMetaData{
		connFact:     connFact,
		target:       target,
		deadline:     deadline,
		protoImports: protoImports,
	}
}

func (s *serviceMetaData) GetServiceMetaDataList(ctx context.Context) (ServiceMetaList, error) {
	conn, err := s.connFact.GetConn(s.target)
	if err != nil {
		return nil, err
	}
	callctx, cancel := context.WithTimeout(ctx, time.Duration(s.deadline)*time.Second)
	defer cancel()
	rc := grpcreflect.NewClientAuto(callctx, conn)

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
			rc = grpcreflect.NewClientAuto(callctx, conn)
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

func (s *serviceMetaData) GetAdditionalFiles() ([]*desc.FileDescriptor, error) {
	return s.serviceMetaBase.GetAdditionalFiles(s.protoImports)
}
