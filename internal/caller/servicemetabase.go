package caller

import (
	"context"
	"errors"
	"fmt"

	"github.com/jhump/protoreflect/desc"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type ServiceMetaData interface {
	GetServiceMetaDataList(context.Context) (ServiceMetaList, error)
	GetAdditionalFiles() ([]*desc.FileDescriptor, error)
}

type ServiceMeta struct {
	Name    string
	Methods []*desc.MethodDescriptor
	File    *desc.FileDescriptor
}

type ServiceMetaList []*ServiceMeta

func (l ServiceMetaList) Files() []*desc.FileDescriptor {
	res := make([]*desc.FileDescriptor, len(l))
	for i, m := range l {
		res[i] = m.File
	}

	return res
}

type serviceMetaBase struct{}

func (s serviceMetaBase) GetAdditionalFiles(protoImports []string) ([]*desc.FileDescriptor, error) {
	if len(protoImports) == 0 {
		return nil, nil
	}
	fileDesc, err := parseProtoFiles(protoImports, nil)
	if err != nil {
		if errors.Is(err, errNoProtoFilesFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("error parsing additional proto files: %w", err)
	}
	return fileDesc, nil
}

func RegisterFiles(fds ...*desc.FileDescriptor) {
	for _, fd := range fds {
		protoFile := fd.UnwrapFile()
		_, err := protoregistry.GlobalFiles.FindFileByPath(protoFile.Path())
		if errors.Is(err, protoregistry.NotFound) {
			protoregistry.GlobalFiles.RegisterFile(protoFile)
		}
	}
}
