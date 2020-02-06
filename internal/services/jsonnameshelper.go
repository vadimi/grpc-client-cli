package services

import "github.com/jhump/protoreflect/desc"

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
func (u *jsonNamesUpdater) updateJSONNames(t *desc.MessageDescriptor) {
	u.walker.Walk(t, func(f *desc.FieldDescriptor) {
		u.updateJSONName(f)
	})
}

func (u *jsonNamesUpdater) updateJSONName(f *desc.FieldDescriptor) {
	fdp := f.AsFieldDescriptorProto()
	if fdp.GetJsonName() == "" {
		cc := toLowerCamelCase(f.GetName())
		fdp.JsonName = &cc
	}
}
