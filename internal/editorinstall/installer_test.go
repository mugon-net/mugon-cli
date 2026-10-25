package editorinstall

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, content := range files {
		if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func zipArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type release struct {
	version     string
	requiresCli string
	archive     []byte
	extension   string
}

func newServer(t *testing.T, platform string, releases ...release) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var index Index
	for _, entry := range releases {
		name := "/assets/editor-" + entry.version + entry.extension
		data := entry.archive
		mux.HandleFunc(name, func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write(data) })
		index.Versions = append(index.Versions, Entry{
			Version:     entry.version,
			RequiresCli: entry.requiresCli,
			Assets:      map[string]Asset{platform: {URL: "SERVER" + name, SHA256: checksum(data)}},
		})
	}
	var server *httptest.Server
	mux.HandleFunc("/editor/index.json", func(writer http.ResponseWriter, _ *http.Request) {
		body, err := json.Marshal(index)
		if err != nil {
			t.Error(err)
		}
		_, _ = writer.Write(bytes.ReplaceAll(body, []byte("SERVER"), []byte(server.URL)))
	})
	server = httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func newInstaller(t *testing.T, server *httptest.Server, cliVersion string) *Installer {
	t.Helper()
	installer := NewInstaller(t.TempDir(), server.URL, cliVersion, nil)
	installer.Platform = "linux-amd64"
	return installer
}

func editorFiles(version string) map[string]string {
	return map[string]string{
		"mugon-editor/mugon-editor":        "binary " + version,
		"mugon-editor/config/example.toml": "x",
	}
}

func TestInstallPicksNewestCompatibleAndSetsCurrent(t *testing.T) {
	server := newServer(t, "linux-amd64",
		release{version: "0.1.0", requiresCli: ">=0.1.0 <1.0.0", archive: tarGz(t, editorFiles("0.1.0")), extension: ".tar.gz"},
		release{version: "0.2.0", requiresCli: ">=0.1.0 <1.0.0", archive: tarGz(t, editorFiles("0.2.0")), extension: ".tar.gz"},
		release{version: "0.3.0", requiresCli: ">=2.0.0", archive: tarGz(t, editorFiles("0.3.0")), extension: ".tar.gz"},
	)
	installer := newInstaller(t, server, "0.4.0")

	version, err := installer.Install("")
	if err != nil || version != "0.2.0" {
		t.Fatalf("got %q, %v", version, err)
	}
	if installer.Current() != "0.2.0" {
		t.Fatalf("current is %q", installer.Current())
	}
	content, err := os.ReadFile(filepath.Join(installer.Root, "0.2.0", "mugon-editor"))
	if err != nil || string(content) != "binary 0.2.0" {
		t.Fatalf("got %q, %v", content, err)
	}
	if _, err := os.Stat(filepath.Join(installer.Root, "0.2.0", "config", "example.toml")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallExplicitVersion(t *testing.T) {
	server := newServer(t, "linux-amd64",
		release{version: "0.1.0", archive: tarGz(t, editorFiles("0.1.0")), extension: ".tar.gz"},
		release{version: "0.2.0", archive: tarGz(t, editorFiles("0.2.0")), extension: ".tar.gz"},
	)
	installer := newInstaller(t, server, "dev")
	if _, err := installer.Install("v0.1.0"); err != nil {
		t.Fatal(err)
	}
	if installer.Current() != "0.1.0" {
		t.Fatalf("current is %q", installer.Current())
	}
	if _, err := installer.Install("9.9.9"); err == nil {
		t.Fatal("expected an error for an unknown version")
	}
}

func TestInstallUnpacksZip(t *testing.T) {
	server := newServer(t, "linux-amd64", release{version: "0.1.0", archive: zipArchive(t, editorFiles("0.1.0")), extension: ".zip"})
	installer := newInstaller(t, server, "dev")
	if _, err := installer.Install(""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(installer.Root, "0.1.0", "mugon-editor")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallRejectsChecksumMismatch(t *testing.T) {
	good := tarGz(t, editorFiles("0.1.0"))
	server := newServer(t, "linux-amd64", release{version: "0.1.0", archive: good, extension: ".tar.gz"})
	installer := newInstaller(t, server, "dev")
	installer.Client = &http.Client{Transport: tamper{base: http.DefaultTransport}}
	if _, err := installer.Install(""); err == nil {
		t.Fatal("expected a checksum error")
	}
	if installer.Current() != "" || len(installer.Installed()) != 0 {
		t.Fatal("nothing must be installed after a failed download")
	}
}

type tamper struct{ base http.RoundTripper }

func (round tamper) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := round.base.RoundTrip(request)
	if err == nil && filepath.Ext(request.URL.Path) != ".json" {
		response.Body = http.NoBody
	}
	return response, err
}

func TestInstallRejectsPathTraversal(t *testing.T) {
	archive := tarGz(t, map[string]string{"mugon-editor/../../escape.txt": "x"})
	server := newServer(t, "linux-amd64", release{version: "0.1.0", archive: archive, extension: ".tar.gz"})
	installer := newInstaller(t, server, "dev")
	if _, err := installer.Install(""); err == nil {
		t.Fatal("expected an error for an unsafe archive")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(installer.Root), "escape.txt")); err == nil {
		t.Fatal("file escaped the install folder")
	}
}

func TestUpdateIsNoopWhenCurrent(t *testing.T) {
	server := newServer(t, "linux-amd64", release{version: "0.1.0", archive: tarGz(t, editorFiles("0.1.0")), extension: ".tar.gz"})
	installer := newInstaller(t, server, "dev")
	if _, err := installer.Install(""); err != nil {
		t.Fatal(err)
	}
	version, err := installer.Update()
	if err != nil || version != "0.1.0" {
		t.Fatalf("got %q, %v", version, err)
	}
}

func TestUninstallFallsBackToNewestRemaining(t *testing.T) {
	server := newServer(t, "linux-amd64",
		release{version: "0.1.0", archive: tarGz(t, editorFiles("0.1.0")), extension: ".tar.gz"},
		release{version: "0.2.0", archive: tarGz(t, editorFiles("0.2.0")), extension: ".tar.gz"},
	)
	installer := newInstaller(t, server, "dev")
	for _, version := range []string{"0.1.0", "0.2.0"} {
		if _, err := installer.Install(version); err != nil {
			t.Fatal(err)
		}
	}
	if err := installer.Uninstall("", false); err != nil {
		t.Fatal(err)
	}
	if installer.Current() != "0.1.0" {
		t.Fatalf("current is %q", installer.Current())
	}
	if err := installer.Uninstall("", true); err != nil {
		t.Fatal(err)
	}
	if installer.Current() != "" || len(installer.Installed()) != 0 {
		t.Fatal("expected everything removed")
	}
}

func TestSatisfies(t *testing.T) {
	cases := []struct {
		cli, requirement string
		want             bool
	}{
		{"0.5.0", ">=0.5.0 <1.0.0", true},
		{"0.4.9", ">=0.5.0 <1.0.0", false},
		{"1.0.0", ">=0.5.0 <1.0.0", false},
		{"1.2.3", "", true},
		{"dev", ">=9.0.0", true},
		{"0.2.0", "=0.2.0", true},
		{"0.2.1", ">0.2.1", false},
	}
	for _, testCase := range cases {
		got, err := satisfies(testCase.cli, testCase.requirement)
		if err != nil || got != testCase.want {
			t.Errorf("satisfies(%q, %q) = %v, %v; want %v", testCase.cli, testCase.requirement, got, err, testCase.want)
		}
	}
	if _, err := satisfies("1.0.0", "~1.0.0"); err == nil {
		t.Error("expected an error for an unsupported comparator")
	}
}

func TestPlatformKey(t *testing.T) {
	if got := PlatformKey("windows", "amd64"); got != "windows-amd64" {
		t.Fatalf("got %q", got)
	}
	if got := PlatformKey("linux", "arm64"); got != "linux-arm64" {
		t.Fatalf("got %q", got)
	}
}
