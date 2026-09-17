package app

import (
	"errors"
	"sort"
	"strings"

	"github.com/vadimi/grpc-client-cli/internal/caller"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// goBackOption is the first entry of the method list; selecting it returns
// to service selection.
const goBackOption = "[..]"

// ResolveService returns the name of the first service whose full name
// contains name (case-insensitive). An empty name never matches.
func ResolveService(services caller.ServiceMetaList, name string) (string, error) {
	if svc, ok := matchService(services, name); ok {
		return svc, nil
	}

	return "", errors.New("service name not found or invalid")
}

// ResolveMethod returns the method of svc whose name equals name
// (case-insensitive). An empty name never matches.
func ResolveMethod(svc *caller.ServiceMeta, name string) (protoreflect.MethodDescriptor, error) {
	if m, ok := matchMethod(svc, name); ok {
		return m, nil
	}

	return nil, errors.New("method name not found or invalid")
}

// findService returns the service meta for a full service name.
func findService(services caller.ServiceMetaList, name string) *caller.ServiceMeta {
	for _, s := range services {
		if s.Name == name {
			return s
		}
	}

	return nil
}

// matchService returns the first service whose full name contains name
// (case-insensitive). An empty name never matches.
func matchService(services caller.ServiceMetaList, name string) (string, bool) {
	if name == "" {
		return "", false
	}

	normalized := strings.ToLower(name)
	for _, s := range services {
		if strings.Contains(strings.ToLower(s.Name), normalized) {
			return s.Name, true
		}
	}

	return "", false
}

// serviceOptions returns all service names sorted ascending,
// case-insensitively.
func serviceOptions(services caller.ServiceMetaList) []string {
	names := make([]string, len(services))
	for i, s := range services {
		names[i] = s.Name
	}

	sort.Slice(names, func(i, j int) bool {
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})

	return names
}

// matchMethod returns the first method of svc whose name equals name
// (case-insensitive). An empty name never matches.
func matchMethod(svc *caller.ServiceMeta, name string) (protoreflect.MethodDescriptor, bool) {
	if svc == nil || name == "" {
		return nil, false
	}

	for _, m := range svc.Methods {
		if strings.EqualFold(string(m.Name()), name) {
			return m, true
		}
	}

	return nil, false
}

// methodOptions returns the go back entry followed by all method names of
// svc sorted ascending, case-insensitively.
func methodOptions(svc *caller.ServiceMeta) []string {
	methodNames := make([]string, len(svc.Methods))
	for i, m := range svc.Methods {
		methodNames[i] = string(m.Name())
	}

	sort.Slice(methodNames, func(i, j int) bool {
		return strings.ToLower(methodNames[i]) < strings.ToLower(methodNames[j])
	})

	return append([]string{goBackOption}, methodNames...)
}
