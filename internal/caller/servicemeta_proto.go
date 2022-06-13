package caller

import (
	"context"
	"fmt"

	"github.com/jhump/protoreflect/desc"
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

func (smp *serviceMetadataProto) GetServiceMetaDataList(ctx context.Context) ([]*ServiceMeta, error) {
	fileDesc, err := parseProtoFiles(smp.protoPath, smp.protoImports)
	if err != nil {
		return nil, fmt.Errorf("error parsing proto files: %w", err)
	}

	res := []*ServiceMeta{}

	for _, fd := range fileDesc {
		for _, svc := range fd.GetServices() {
			svcData := &ServiceMeta{
				File:    svc.GetFile(),
				Name:    svc.GetFullyQualifiedName(),
				Methods: svc.GetMethods(),
			}

			for _, m := range svcData.Methods {
				u := newJsonNamesUpdater()
				u.updateJSONNames(m.GetInputType())
				u.updateJSONNames(m.GetOutputType())
			}
			res = append(res, svcData)
		}
	}

	return res, nil
}

func (smp *serviceMetadataProto) GetAdditionalFiles() ([]*desc.FileDescriptor, error) {
	return smp.serviceMetaBase.GetAdditionalFiles(smp.protoImports)
}
