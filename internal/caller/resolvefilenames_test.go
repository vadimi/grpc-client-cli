package caller

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeFile(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveFilenamesNoImportPaths(t *testing.T) {
	got, err := resolveFilenames(nil, "a.proto", "dir/b.proto")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "a.proto,dir/b.proto" {
		t.Errorf("unexpected result: %v", got)
	}

	abs, err := filepath.Abs("a.proto")
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolveFilenames(nil, abs)
	if !errors.Is(err, errNoImportPathsForAbsoluteFilePath) {
		t.Errorf("expected errNoImportPathsForAbsoluteFilePath, got %v", err)
	}
}

func TestResolveFilenames(t *testing.T) {
	root := t.TempDir()
	importDir := filepath.Join(root, "protos")
	otherDir := filepath.Join(root, "other")
	outside := filepath.Join(root, "outside", "x.proto")
	writeFile(t, filepath.Join(importDir, "a.proto"), "")
	writeFile(t, filepath.Join(importDir, "sub", "b.proto"), "")
	writeFile(t, filepath.Join(otherDir, "c.proto"), "")
	writeFile(t, outside, "")

	t.Run("relativeToImportPath", func(t *testing.T) {
		got, err := resolveFilenames([]string{importDir}, "a.proto", "sub/b.proto")
		if err != nil {
			t.Fatal(err)
		}
		if want := "a.proto,sub/b.proto"; strings.Join(got, ",") != want {
			t.Errorf("expected %s, got %v", want, got)
		}
	})

	t.Run("absolutePath", func(t *testing.T) {
		got, err := resolveFilenames([]string{importDir}, filepath.Join(importDir, "sub", "b.proto"))
		if err != nil {
			t.Fatal(err)
		}
		if got[0] != "sub/b.proto" {
			t.Errorf("unexpected result: %v", got)
		}
	})

	t.Run("multipleImportPathsKeepOrder", func(t *testing.T) {
		got, err := resolveFilenames([]string{importDir, otherDir},
			filepath.Join(otherDir, "c.proto"), "a.proto")
		if err != nil {
			t.Fatal(err)
		}
		if want := "c.proto,a.proto"; strings.Join(got, ",") != want {
			t.Errorf("expected %s, got %v", want, got)
		}
	})

	t.Run("relativeToWorkingDir", func(t *testing.T) {
		t.Chdir(root)
		got, err := resolveFilenames([]string{importDir}, "./protos/sub/b.proto")
		if err != nil {
			t.Fatal(err)
		}
		if got[0] != "sub/b.proto" {
			t.Errorf("unexpected result: %v", got)
		}
	})

	t.Run("dotDotRelativeToWorkingDir", func(t *testing.T) {
		t.Chdir(otherDir)
		got, err := resolveFilenames([]string{importDir}, "../protos/a.proto")
		if err != nil {
			t.Fatal(err)
		}
		if got[0] != "a.proto" {
			t.Errorf("unexpected result: %v", got)
		}
	})

	t.Run("outsideImportPaths", func(t *testing.T) {
		_, err := resolveFilenames([]string{importDir}, outside)
		if err == nil || !strings.Contains(err.Error(), "does not reside in any import path") {
			t.Errorf("expected 'does not reside' error, got %v", err)
		}
	})

	t.Run("nonexistentRelativeFile", func(t *testing.T) {
		t.Chdir(root)
		_, err := resolveFilenames([]string{importDir}, "missing.proto")
		if err == nil {
			t.Error("expected error for file not in any import path")
		}
	})
}

func TestResolveFilenamesSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	writeFile(t, filepath.Join(realDir, "a.proto"), "")
	link := filepath.Join(root, "link")
	if err := os.Symlink(realDir, link); err != nil {
		t.Fatal(err)
	}

	got, err := resolveFilenames([]string{realDir}, filepath.Join(link, "a.proto"))
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != "a.proto" {
		t.Errorf("unexpected result: %v", got)
	}
}

func TestCanonicalizeNonexistentTail(t *testing.T) {
	root := t.TempDir()
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := canonicalize(filepath.Join(root, "no", "such", "file.proto"))
	if err != nil {
		t.Fatal(err)
	}
	if want = filepath.Join(want, "no", "such", "file.proto"); got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestIsDescendant(t *testing.T) {
	sep := string(filepath.Separator)
	dir := sep + "a" + sep + "b"
	tests := []struct {
		file string
		want bool
	}{
		{dir + sep + "c.proto", true},
		{dir + sep + "x" + sep + "c.proto", true},
		{sep + "a" + sep + "bc" + sep + "c.proto", false},
		{sep + "a" + sep + "c.proto", false},
		{sep + "z.proto", false},
	}
	for _, tt := range tests {
		if got := isDescendant(dir, tt.file); got != tt.want {
			t.Errorf("isDescendant(%s, %s) = %v, want %v", dir, tt.file, got, tt.want)
		}
	}
}
