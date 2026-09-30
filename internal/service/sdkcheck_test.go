package service

import (
	"os"
	"path/filepath"
	"testing"
)

func writeDist(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestFindSdkVersions(t *testing.T) {
	dir := writeDist(t, map[string]string{
		"index.html":       "<html></html>",
		"assets/index.js":  `var n="mugon-sdk-version:0.1.2";`,
		"assets/other.txt": "mugon-sdk-version:9.9.9",
	})
	versions, err := FindSdkVersions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 || versions[0] != "0.1.2" {
		t.Fatalf("versions = %v", versions)
	}
}

func TestSdkLoadedFirst(t *testing.T) {
	sdk := `var n="mugon-sdk-version:0.1.0";`
	cases := map[string]map[string]string{
		"external sdk first": {"index.html": `<script src="/mugon.iife.js"></script><script src="game.js"></script>`, "mugon.iife.js": sdk, "game.js": ""},
		"inline sdk first":   {"index.html": `<script>` + sdk + `</script>`},
		"bundled first":      {"index.html": `<script type="module" src="./assets/index.js"></script>`, "assets/index.js": sdk},
	}
	for name, files := range cases {
		if !SdkLoadedFirst(writeDist(t, files)) {
			t.Errorf("%s: expected SDK to be detected first", name)
		}
	}
	notFirst := writeDist(t, map[string]string{
		"index.html":    `<script src="game.js"></script><script src="mugon.iife.js"></script>`,
		"game.js":       "",
		"mugon.iife.js": sdk,
	})
	if SdkLoadedFirst(notFirst) {
		t.Error("SDK loaded second must not count as first")
	}
}
