package command

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mugon-net/cli/internal/model"
)

func TestFindEditorPrefersPath(t *testing.T) {
	lookPath := func(name string) (string, error) { return "/usr/bin/" + name, nil }
	path, err := findEditor(lookPath, t.TempDir())
	if err != nil || path != "/usr/bin/mugon-editor" {
		t.Fatalf("got %q, %v", path, err)
	}
}

func TestFindEditorFallsBackToInstallDir(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".mugon", "editor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := "mugon-editor"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(dir, name)
	if err := os.WriteFile(binary, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	lookPath := func(string) (string, error) { return "", errors.New("not found") }
	path, err := findEditor(lookPath, home)
	if err != nil || path != binary {
		t.Fatalf("got %q, %v", path, err)
	}
}

func TestFindEditorNotInstalled(t *testing.T) {
	lookPath := func(string) (string, error) { return "", errors.New("not found") }
	_, err := findEditor(lookPath, t.TempDir())
	var notFound model.EditorNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected EditorNotFoundError, got %v", err)
	}
}
