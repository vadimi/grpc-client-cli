package caller

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParseProtoFilesNoProtos(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "readme.txt"), "")

	_, err := parseProtoFiles([]string{dir}, nil)
	if !errors.Is(err, errNoProtoFilesFound) {
		t.Errorf("expected errNoProtoFilesFound, got %v", err)
	}
}

func TestParseProtoFilesSyntaxError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bad.proto"), `syntax = "proto3"; message {`)

	_, err := parseProtoFiles([]string{dir}, nil)
	if err == nil {
		t.Fatal("expected syntax error")
	}
}

func TestParseProtoFilesMissingImport(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.proto"), `syntax = "proto3";
import "nope/missing.proto";
message A {}`)

	_, err := parseProtoFiles([]string{dir}, nil)
	if err == nil || !strings.Contains(err.Error(), "missing.proto") {
		t.Errorf("expected missing import error, got %v", err)
	}
}

func TestParseProtoFilesStandardImports(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.proto"), `syntax = "proto3";
import "google/protobuf/timestamp.proto";
import "google/protobuf/any.proto";
message A {
  google.protobuf.Timestamp ts = 1;
  google.protobuf.Any any = 2;
}`)

	files, err := parseProtoFiles([]string{dir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Messages().Len() != 1 {
		t.Errorf("unexpected parse result: %v", files)
	}
}

func TestParseProtoFilesImportFromAdditionalDir(t *testing.T) {
	files, err := parseProtoFiles(
		[]string{"../../testdata/testapi/withthirdparty"},
		[]string{"../../testdata/testapi/third_party"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	// without the import dir the dependency can't be resolved
	if _, err = parseProtoFiles([]string{"../../testdata/testapi/withthirdparty"}, nil); err == nil {
		t.Error("expected error when import dir is missing")
	}
}

func TestParseProtoFilesNoSourceInfo(t *testing.T) {
	files, err := parseProtoFiles([]string{"../../testdata/testapi/single"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if files[0].SourceLocations().Len() != 0 {
		t.Error("expected no source info")
	}
}

func TestFindProtoFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.proto"), "")
	writeFile(t, filepath.Join(dir, "sub", "b.proto"), "")
	writeFile(t, filepath.Join(dir, "sub", "notes.txt"), "")
	writeFile(t, filepath.Join(dir, "single.proto"), "")

	t.Run("directoryWalkedRecursively", func(t *testing.T) {
		got, err := findProtoFiles([]string{dir})
		if err != nil {
			t.Fatal(err)
		}
		slices.Sort(got)
		want := []string{
			filepath.Join(dir, "a.proto"),
			filepath.Join(dir, "single.proto"),
			filepath.Join(dir, "sub", "b.proto"),
		}
		if !slices.Equal(got, want) {
			t.Errorf("expected %v, got %v", want, got)
		}
	})

	t.Run("fileUsesBaseName", func(t *testing.T) {
		got, err := findProtoFiles([]string{filepath.Join(dir, "single.proto")})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, []string{"single.proto"}) {
			t.Errorf("unexpected result: %v", got)
		}
	})

	t.Run("otherExtensionSkipped", func(t *testing.T) {
		got, err := findProtoFiles([]string{filepath.Join(dir, "sub", "notes.txt")})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("expected nothing, got %v", got)
		}
	})

	t.Run("nonexistentDirectory", func(t *testing.T) {
		if _, err := findProtoFiles([]string{filepath.Join(dir, "missing")}); err == nil {
			t.Error("expected error")
		}
	})
}
