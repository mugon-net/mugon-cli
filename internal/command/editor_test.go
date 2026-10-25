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

func TestFindEditorFallsBackToCurrentVersion(t *testing.T) {
	home := t.TempDir()
	editorDir := filepath.Join(home, ".mugon", "editor")
	dir := filepath.Join(editorDir, "0.2.0")
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

	if err := os.WriteFile(filepath.Join(editorDir, "current"), []byte("0.2.0"), 0o644); err != nil {
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

func TestFindEditorIgnoresOldFlatInstall(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".mugon", "editor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	name := "mugon-editor"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	lookPath := func(string) (string, error) { return "", errors.New("not found") }
	_, err := findEditor(lookPath, home)
	var notFound model.EditorNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected EditorNotFoundError, got %v", err)
	}
}

func TestParseManageArgs(t *testing.T) {
	requested, all, err := parseManageArgs("install", []string{"--version", "0.2.0"})
	if err != nil || requested != "0.2.0" || all {
		t.Fatalf("got %q %v %v", requested, all, err)
	}
	_, all, err = parseManageArgs("uninstall", []string{"--all"})
	if err != nil || !all {
		t.Fatalf("got %v %v", all, err)
	}
	for _, bad := range []struct {
		action string
		args   []string
	}{{"update", []string{"--version", "1.0.0"}}, {"install", []string{"--all"}}, {"install", []string{"--version"}}, {"install", []string{"x"}}} {
		if _, _, err := parseManageArgs(bad.action, bad.args); err == nil {
			t.Errorf("expected an error for %s %v", bad.action, bad.args)
		}
	}
}
