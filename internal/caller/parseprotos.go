package caller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"path/filepath"

	"github.com/bufbuild/protocompile"
	clifs "github.com/vadimi/grpc-client-cli/internal/fs"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var errNoProtoFilesFound = errors.New("no proto files found")

func parseProtoFiles(protoDirs []string, protoImports []string) ([]protoreflect.FileDescriptor, error) {
	protofiles, err := findProtoFiles(protoDirs)
	if err != nil {
		return nil, err
	}

	if len(protofiles) == 0 {
		return nil, fmt.Errorf("%w: %s", errNoProtoFilesFound, protoDirs)
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

	resolvedFiles, err := resolveFilenames(importPaths, protofiles...)
	if err != nil {
		return nil, err
	}

	resolver := protocompile.WithStandardImports(&protocompile.SourceResolver{
		ImportPaths: importPaths,
		Accessor: func(filename string) (io.ReadCloser, error) {
			return clifs.NewFileReader(filename)
		},
	})

	// MaxParallelism 1 and no source info match protoparse defaults
	c := protocompile.Compiler{
		Resolver:       resolver,
		MaxParallelism: 1,
		SourceInfoMode: protocompile.SourceInfoNone,
	}

	files, err := c.Compile(context.Background(), resolvedFiles...)
	if err != nil {
		return nil, err
	}

	result := make([]protoreflect.FileDescriptor, len(files))
	for i, fd := range files {
		result[i] = fd
	}
	return result, nil
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

		err := filepath.Walk(p, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if filepath.Ext(info.Name()) == ".proto" {
				protofiles = append(protofiles, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return protofiles, nil
}
