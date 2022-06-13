package caller

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/vadimi/grpc-client-cli/internal/fs"
)

func parseProtoFiles(protoDirs []string, protoImports []string) ([]*desc.FileDescriptor, error) {
	protofiles, err := findProtoFiles(protoDirs)
	if err != nil {
		return nil, err
	}

	if len(protofiles) == 0 {
		return nil, fmt.Errorf("no proto files found in %s", protoDirs)
	}

	importPaths := []string{}
	for _, pd := range protoDirs {
		if path.Ext(pd) != "" {
			pd = path.Dir(pd)
		}

		importPaths = append(importPaths, pd)
	}

	// additional directories to look for dependencies
	importPaths = append(importPaths, protoImports...)

	p := protoparse.Parser{
		ImportPaths: importPaths,
		Accessor: func(filename string) (io.ReadCloser, error) {
			return fs.NewFileReader(filename)
		},
	}

	resolvedFiles, err := protoparse.ResolveFilenames(importPaths, protofiles...)
	if err != nil {
		return nil, err
	}

	return p.ParseFiles(resolvedFiles...)
}

func findProtoFiles(paths []string) ([]string, error) {
	protofiles := []string{}
	for _, p := range paths {
		ext := path.Ext(p)
		if ext == ".proto" {
			protofiles = append(protofiles, filepath.Base(p))
			continue
		}

		// non proto extension - skip
		if ext != "" {
			continue
		}

		files, err := os.ReadDir(p)
		if err != nil {
			return nil, err
		}

		for _, f := range files {
			if filepath.Ext(f.Name()) == ".proto" {
				protofiles = append(protofiles, f.Name())
			}
		}
	}

	return protofiles, nil
}
