package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestWriteGlobalConfigKeepsEditorTable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	path := filepath.Join(home, ".mugon", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[credentials]\ngame = \"key\"\n\n[editor]\nlibrary = \"D:/lib\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var config GlobalConfig
	if _, err := toml.DecodeFile(path, &config); err != nil {
		t.Fatal(err)
	}
	delete(config.Credentials, "game")
	if err := WriteGlobalConfig(&config); err != nil {
		t.Fatal(err)
	}

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "library = \"D:/lib\"") {
		t.Fatalf("editor table was dropped:\n%s", written)
	}
}
