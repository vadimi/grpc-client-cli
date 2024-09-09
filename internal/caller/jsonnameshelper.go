package caller

import (
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type jsonNamesUpdater struct {
	walker *FieldWalker
}

func newJsonNamesUpdater() *jsonNamesUpdater {
	return &jsonNamesUpdater{
		walker: NewFieldWalker(),
	}
}

// updateJSONNames sets JsonName property to camelCased original name
// as those properties received through reflection or other non-go means don't have it set
func (u *jsonNamesUpdater) updateJSONNames(t protoreflect.MessageDescriptor) {
	u.walker.Walk(t, func(f protoreflect.FieldDescriptor) {
		fd := protodesc.ToFieldDescriptorProto(f)
		u.updateJSONName(fd)
	})
}

func (u *jsonNamesUpdater) updateJSONName(fdp *descriptorpb.FieldDescriptorProto) {
	if fdp.GetJsonName() == "" {
		cc := toLowerCamelCase(fdp.GetName())
		fdp.JsonName = &cc
	}
}
