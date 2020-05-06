package caller

import "github.com/jhump/protoreflect/desc"

// FieldWalker walks fields message fields tree calling func for every field
type FieldWalker struct {
	processed map[string]struct{}
}

func NewFieldWalker() *FieldWalker {
	return &FieldWalker{
		processed: map[string]struct{}{},
	}
}

func (fw *FieldWalker) Walk(md *desc.MessageDescriptor, walkFn func(*desc.FieldDescriptor)) {
	if md == nil {
		return
	}
	if _, ok := fw.processed[md.GetName()]; ok {
		return
	}
	fw.processed[md.GetName()] = struct{}{}
	for _, f := range md.GetFields() {
		fw.Walk(f.GetMessageType(), walkFn)
		walkFn(f)
	}
}
