package caller

import (
	"google.golang.org/protobuf/reflect/protoreflect"
)

// FieldWalker walks fields message fields tree calling func for every field
type FieldWalker struct {
	processed map[protoreflect.Name]struct{}
}

func NewFieldWalker() *FieldWalker {
	return &FieldWalker{
		processed: map[protoreflect.Name]struct{}{},
	}
}

func (fw *FieldWalker) Walk(md protoreflect.MessageDescriptor, walkFn func(protoreflect.FieldDescriptor)) {
	if md == nil {
		return
	}
	if _, ok := fw.processed[md.Name()]; ok {
		return
	}
	fw.processed[md.Name()] = struct{}{}
	for i := 0; i < md.Fields().Len(); i++ {
		f := md.Fields().Get(i)
		if f.Kind() == protoreflect.MessageKind {
			fw.Walk(f.Message(), walkFn)
		}
		walkFn(f)
	}
}
