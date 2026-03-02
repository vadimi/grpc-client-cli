package caller

import (
	"context"
	"errors"
	"fmt"
	"os"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func init() {
	// don't panic on proto registration conflicts, return errors instead
	os.Setenv("GOLANG_PROTOBUF_REGISTRATION_CONFLICT", "warn")
}

type ServiceMetaData interface {
	GetServiceMetaDataList(context.Context) (ServiceMetaList, error)
	GetAdditionalFiles() ([]protoreflect.FileDescriptor, error)
}

type ServiceMeta struct {
	Name    string
	Methods []protoreflect.MethodDescriptor
	File    protoreflect.FileDescriptor
}

type ServiceMetaList []*ServiceMeta

func (l ServiceMetaList) Files() []protoreflect.FileDescriptor {
	res := make([]protoreflect.FileDescriptor, len(l))
	for i, m := range l {
		res[i] = m.File
	}

	return res
}

type serviceMetaBase struct{}

func (s serviceMetaBase) GetAdditionalFiles(protoImports []string) ([]protoreflect.FileDescriptor, error) {
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

func RegisterFiles(fds ...protoreflect.FileDescriptor) error {
	errs := []error{}
	for _, fd := range fds {
		_, err := protoregistry.GlobalFiles.FindFileByPath(fd.Path())
		if errors.Is(err, protoregistry.NotFound) && shouldRegister(fd) {
			if err := protoregistry.GlobalFiles.RegisterFile(fd); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func shouldRegister(fd protoreflect.FileDescriptor) bool {
	for i := 0; i < fd.Messages().Len(); i++ {
		msg := fd.Messages().Get(i)
		_, err := protoregistry.GlobalTypes.FindMessageByURL(string(msg.FullName()))
		if errors.Is(err, protoregistry.NotFound) {
			return true
		}
	}

	return false
}
