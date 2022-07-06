package caller

import "github.com/jhump/protoreflect/desc"

// FileDescCache stores unique processed protobuf file descriptors to reuse them in other parts of the app
type FileDescCache struct {
	keys  map[string]struct{}
	files []*desc.FileDescriptor
}

func NewFileDescCache(files []*desc.FileDescriptor) *FileDescCache {
	c := &FileDescCache{
		keys:  map[string]struct{}{},
		files: []*desc.FileDescriptor{},
	}

	for _, f := range files {
		c.Add(f)
	}

	return c
}

func (fdc *FileDescCache) Add(d *desc.FileDescriptor) {
	key := d.GetFullyQualifiedName()
	if _, ok := fdc.keys[key]; !ok {
		fdc.keys[key] = struct{}{}
		fdc.files = append(fdc.files, d)
	}
}

func (fdc *FileDescCache) Files() []*desc.FileDescriptor {
	return fdc.files
}
