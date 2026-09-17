package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vadimi/grpc-client-cli/internal/caller"
	grpc_testing "github.com/vadimi/grpc-client-cli/internal/testing/grpc_testing"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func testServiceList() caller.ServiceMetaList {
	return caller.ServiceMetaList{
		{Name: "grpc_client_cli.testing.TestService", Methods: testMethods()},
		{Name: "grpc.health.v1.Health"},
		{Name: "aaa.FirstService"},
		{Name: "bbb.SecondService"},
	}
}

func testMethods() []protoreflect.MethodDescriptor {
	svc := grpc_testing.File_test_proto.Services().ByName("TestService")
	methods := make([]protoreflect.MethodDescriptor, svc.Methods().Len())
	for i := 0; i < svc.Methods().Len(); i++ {
		methods[i] = svc.Methods().Get(i)
	}

	return methods
}

func TestResolveService(t *testing.T) {
	services := testServiceList()

	cases := []struct {
		name        string
		serviceName string
		expected    string
		errExpected bool
		errMessage  string
	}{
		{
			name:        "PartialMatch",
			serviceName: "testservice",
			expected:    "grpc_client_cli.testing.TestService",
		},
		{
			name:        "CaseInsensitiveMatch",
			serviceName: "HEALTH",
			expected:    "grpc.health.v1.Health",
		},
		{
			name:        "FirstMatchWins",
			serviceName: "service",
			expected:    "grpc_client_cli.testing.TestService",
		},
		{
			name:        "NoMatch",
			serviceName: "missing",
			errExpected: true,
			errMessage:  "service name not found or invalid",
		},
		{
			name:        "EmptyName",
			serviceName: "",
			errExpected: true,
			errMessage:  "service name not found or invalid",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, err := ResolveService(services, c.serviceName)
			if c.errExpected {
				require.Error(t, err)
				assert.EqualError(t, err, c.errMessage)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, c.expected, svc)
		})
	}
}

func TestFindService(t *testing.T) {
	services := testServiceList()

	svc := findService(services, "grpc.health.v1.Health")
	require.NotNil(t, svc)
	assert.Equal(t, "grpc.health.v1.Health", svc.Name)

	assert.Nil(t, findService(services, "missing"))
	assert.Nil(t, findService(services, ""))
}

func TestServiceOptionsSorted(t *testing.T) {
	opts := serviceOptions(testServiceList())

	expected := []string{
		"aaa.FirstService",
		"bbb.SecondService",
		"grpc.health.v1.Health",
		"grpc_client_cli.testing.TestService",
	}
	assert.Equal(t, expected, opts)
}

func TestResolveMethod(t *testing.T) {
	svc := &caller.ServiceMeta{Name: "grpc_client_cli.testing.TestService", Methods: testMethods()}

	cases := []struct {
		name        string
		methodName  string
		expected    string
		errExpected bool
		errMessage  string
	}{
		{
			name:       "ExactMatch",
			methodName: "UnaryCall",
			expected:   "UnaryCall",
		},
		{
			name:       "CaseInsensitiveMatch",
			methodName: "unarycall",
			expected:   "UnaryCall",
		},
		{
			name:        "PartialNameDoesNotMatch",
			methodName:  "unary",
			errExpected: true,
			errMessage:  "method name not found or invalid",
		},
		{
			name:        "NoMatch",
			methodName:  "MissingCall",
			errExpected: true,
			errMessage:  "method name not found or invalid",
		},
		{
			name:        "EmptyName",
			methodName:  "",
			errExpected: true,
			errMessage:  "method name not found or invalid",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, err := ResolveMethod(svc, c.methodName)
			if c.errExpected {
				require.Error(t, err)
				assert.EqualError(t, err, c.errMessage)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, m)
			assert.Equal(t, c.expected, string(m.Name()))
		})
	}
}

func TestResolveMethodNilService(t *testing.T) {
	_, err := ResolveMethod(nil, "UnaryCall")
	require.Error(t, err)
	assert.EqualError(t, err, "method name not found or invalid")
}

func TestMethodOptions(t *testing.T) {
	svc := &caller.ServiceMeta{Name: "grpc_client_cli.testing.TestService", Methods: testMethods()}

	opts := methodOptions(svc)

	expected := []string{
		"[..]",
		"EmptyCall",
		"FullDuplexCall",
		"HalfDuplexCall",
		"StreamingInputCall",
		"StreamingOutputCall",
		"UnaryAny",
		"UnaryCall",
		"UnaryUpdateCall",
	}
	assert.Equal(t, expected, opts)
}
