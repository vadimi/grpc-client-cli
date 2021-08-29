package caller

import (
	"context"
	"testing"
)

func TestMetaDataListSingleFile(t *testing.T) {
	tests := []struct {
		name      string
		protoPath string
	}{
		{name: "protoDirectory", protoPath: "../../testdata/testapi/single"},
		{name: "protoFile", protoPath: "../../testdata/testapi/single/user-service.proto"},
	}

	expectedMethods := []string{"GetUser", "GetAllUsers", "SaveAllUsers", "RequestUsers"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svcMeta := NewServiceMetadataProto([]string{tt.protoPath}, nil)
			services, err := svcMeta.GetServiceMetaDataList(context.Background())
			if err != nil {
				t.Error(err)
				return
			}

			expectedSvc := "testservice.UserService"
			s := findSvc(services, expectedSvc)
			if s == nil {
				t.Errorf("service '%s' not found", expectedSvc)
				return
			}

			if len(s.Methods) != len(expectedMethods) {
				t.Errorf("wrong number of methods, expected: %d, got: %d", len(expectedMethods), len(s.Methods))
				return
			}

			for i := range s.Methods {
				mn := s.Methods[i].GetName()
				if !stringInArray(expectedMethods, mn) {
					t.Errorf("unexpected method: %s", mn)
				}
			}
		})
	}
}

func TestMetaDataListMultipleFiles(t *testing.T) {
	tests := []struct {
		name             string
		protoPath        []string
		thirdParty       []string
		expectedMethods  []string
		expectedServices []string
	}{
		{
			name:             "protoDirectory",
			protoPath:        []string{"../../testdata/testapi/multiple"},
			expectedServices: []string{"testservice.Service1", "testservice.Service2"},
		},
		{
			name: "protoFile",
			protoPath: []string{
				"../../testdata/testapi/multiple/service1.proto",
				"../../testdata/testapi/multiple/service2.proto",
			},
			expectedServices: []string{"testservice.Service1", "testservice.Service2"},
		},
		{
			name:             "withThirdParty",
			protoPath:        []string{"../../testdata/testapi/withthirdparty"},
			expectedServices: []string{"testservice.Service"},
			expectedMethods:  []string{"GetData"},
			thirdParty:       []string{"../../testdata/testapi/third_party"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svcMeta := NewServiceMetadataProto(tt.protoPath, tt.thirdParty)
			services, err := svcMeta.GetServiceMetaDataList(context.Background())
			if err != nil {
				t.Error(err)
				return
			}

			for _, expectedSvc := range tt.expectedServices {
				s := findSvc(services, expectedSvc)
				if s == nil {
					t.Errorf("service '%s' not found", expectedSvc)
					return
				}

				if len(tt.expectedMethods) == 0 {
					continue
				}

				if len(s.Methods) != len(tt.expectedMethods) {
					t.Errorf("wrong number of methods, expected: %d, got: %d", len(tt.expectedMethods), len(s.Methods))
					return
				}

				for i := range s.Methods {
					mn := s.Methods[i].GetName()
					if !stringInArray(tt.expectedMethods, mn) {
						t.Errorf("unexpected method: %s", mn)
					}
				}

			}
		})
	}
}

func findSvc(services []*ServiceMeta, name string) *ServiceMeta {
	for _, s := range services {
		if s.Name == name {
			return s
		}
	}

	return nil
}

func stringInArray(arr []string, s string) bool {
	for i := range arr {
		if arr[i] == s {
			return true
		}
	}

	return false
}
