package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/pkg/errors"
	rpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

var (
	ErrNotFound = errors.New("grpc element not found")
)

type ServiceMetaData struct {
	connFact *GrpcConnFactory
}

type ServiceMeta struct {
	Name    string
	Methods []*desc.MethodDescriptor
	File    *desc.FileDescriptor
}

func NewServiceMetaData(connFact *GrpcConnFactory) *ServiceMetaData {
	return &ServiceMetaData{connFact}
}

func (s *ServiceMetaData) GetFileDescriptor(target, serviceName string) (*desc.FileDescriptor, error) {
	conn, err := s.connFact.GetConn(target)
	if err != nil {
		return nil, err
	}
	rpbclient := rpb.NewServerReflectionClient(conn)
	rc := grpcreflect.NewClient(context.Background(), rpbclient)
	defer rc.Reset()

	svc, err := rc.ResolveService(serviceName)
	if err != nil {
		if grpcreflect.IsElementNotFoundError(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return svc.GetFile(), nil
}

func (s *ServiceMetaData) GetOutputMessageDescriptor(target, service, methodName string) (*desc.MessageDescriptor, error) {
	fd, err := s.GetFileDescriptor(target, service)
	if err != nil {
		return nil, err
	}

	sd := fd.FindService(service)
	if sd == nil {
		return nil, fmt.Errorf("service %s not found", service)
	}

	md := sd.FindMethodByName(methodName)
	if md == nil {
		return nil, fmt.Errorf("method %s not found", methodName)
	}

	return md.GetOutputType(), nil
}

func (s *ServiceMetaData) GetOutputMessageDescriptorParse(target, fullMethodName string) (*desc.MessageDescriptor, error) {
	normMethod := strings.Trim(fullMethodName, "/")
	i := strings.LastIndex(normMethod, "/")
	service := normMethod[0:i]
	methodName := normMethod[i+1:]

	return s.GetOutputMessageDescriptor(target, service, methodName)
}

func (s *ServiceMetaData) GetServiceMetaDataList(target string, deadline int) ([]*ServiceMeta, error) {
	conn, err := s.connFact.GetConn(target)
	if err != nil {
		return nil, err
	}
	rpbclient := rpb.NewServerReflectionClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(deadline)*time.Second)
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

func (s *ServiceMetaData) GetMethodDescriptor(target, service, methodName string) (*desc.MethodDescriptor, error) {
	fd, err := s.GetFileDescriptor(target, service)
	if err != nil {
		return nil, err
	}
	sd := fd.FindService(service)
	if sd == nil {
		return nil, fmt.Errorf("service %s not found", service)
	}

	md := sd.FindMethodByName(methodName)
	if md == nil {
		return nil, fmt.Errorf("method %s not found", methodName)
	}

	return md, nil
}
