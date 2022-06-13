package caller

import (
	"context"
	"fmt"

	"github.com/jhump/protoreflect/desc"
)

type ServiceMetaData interface {
	GetServiceMetaDataList(context.Context) ([]*ServiceMeta, error)
	GetAdditionalFiles() ([]*desc.FileDescriptor, error)
}

type ServiceMeta struct {
	Name    string
	Methods []*desc.MethodDescriptor
	File    *desc.FileDescriptor
}

type serviceMetaBase struct{}

func (s serviceMetaBase) GetAdditionalFiles(protoImports []string) ([]*desc.FileDescriptor, error) {
	if len(protoImports) == 0 {
		return nil, nil
	}
	fileDesc, err := parseProtoFiles(protoImports, nil)
	if err != nil {
		return nil, fmt.Errorf("error parsing additional proto files: %w", err)
	}
	return fileDesc, nil
}
