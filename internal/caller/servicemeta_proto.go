package caller

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
)

type serviceMetadataProto struct {
	protoPath    []string
	protoImports []string

	serviceMetaBase
}

// NewServiceMetadataProto returns new instance of ServiceMetaData
// that reads service metadata from proto files on disk.
// protoPath - proto files or directories of proto files
// protoImports - additional directories to search for proto files dependencies
func NewServiceMetadataProto(protoPath, protoImports []string) ServiceMetaData {
	return &serviceMetadataProto{
		protoPath:    protoPath,
		protoImports: protoImports,
	}
}

func (smp *serviceMetadataProto) GetServiceMetaDataList(ctx context.Context) (ServiceMetaList, error) {
	fileDesc, err := parseProtoFiles(smp.protoPath, smp.protoImports)
	if err != nil {
		return nil, fmt.Errorf("error parsing proto files: %w", err)
	}

	res := []*ServiceMeta{}

	for _, fd := range fileDesc {
		for i := 0; i < fd.Services().Len(); i++ {
			svc := fd.Services().Get(i)

			methods := make([]protoreflect.MethodDescriptor, svc.Methods().Len())
			for j := 0; j < svc.Methods().Len(); j++ {
				methods[j] = svc.Methods().Get(j)
			}

			svcData := &ServiceMeta{
				File:    fd,
				Name:    string(svc.FullName()),
				Methods: methods,
			}

			for _, m := range svcData.Methods {
				u := newJsonNamesUpdater()
				u.updateJSONNames(m.Input())
				u.updateJSONNames(m.Output())
			}
			res = append(res, svcData)
		}
	}

	return res, nil
}

func (smp *serviceMetadataProto) GetAdditionalFiles() ([]protoreflect.FileDescriptor, error) {
	return smp.serviceMetaBase.GetAdditionalFiles(smp.protoImports)
}
