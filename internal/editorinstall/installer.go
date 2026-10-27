package editorinstall

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	DefaultToolsURL = "https://tools.mugon.net"
	currentFileName = "current"
	archiveRootDir  = "mugon-editor"
	maxArchiveBytes = 1 << 30
)

// Installer manages ~/.mugon/editor/<version>/ folders and the `current` pointer file that names the active one.
type Installer struct {
	// Root is the editor folder, normally ~/.mugon/editor.
	Root string
	// ToolsURL is the base URL of the tools host; the index lives at <ToolsURL>/editor/index.json.
	ToolsURL   string
	CliVersion string
	Platform   string
	Client     *http.Client
	Out        io.Writer
}

func NewInstaller(home, toolsURL, cliVersion string, out io.Writer) *Installer {
	if toolsURL == "" {
		toolsURL = DefaultToolsURL
	}
	return &Installer{
		Root:       filepath.Join(home, ".mugon", "editor"),
		ToolsURL:   strings.TrimRight(toolsURL, "/"),
		CliVersion: cliVersion,
		Platform:   CurrentPlatformKey(),
		Client:     &http.Client{Timeout: 10 * time.Minute},
		Out:        out,
	}
}

func (installer *Installer) println(format string, args ...any) {
	if installer.Out != nil {
		_, _ = fmt.Fprintf(installer.Out, format+"\n", args...)
	}
}

// Current returns the active version, or "" when none is installed.
func (installer *Installer) Current() string {
	content, err := os.ReadFile(filepath.Join(installer.Root, currentFileName))
	if err != nil {
		return ""
	}
	version := strings.TrimSpace(string(content))
	if _, err := parseSemver(version); err != nil {
		return ""
	}
	return version
}

func (installer *Installer) versionDir(version string) string {
	return filepath.Join(installer.Root, version)
}

// Installed lists the installed versions, oldest first.
func (installer *Installer) Installed() []string {
	entries, err := os.ReadDir(installer.Root)
	if err != nil {
		return nil
	}
	var versions []string
	for _, entry := range entries {
		if _, err := parseSemver(entry.Name()); err == nil && entry.IsDir() {
			versions = append(versions, entry.Name())
		}
	}
	sort.Slice(versions, func(left, right int) bool {
		return mustParse(versions[left]).compare(mustParse(versions[right])) < 0
	})
	return versions
}

// Install downloads the requested version (default: the newest compatible one), installs it next to the others and
// makes it current. It returns the installed version.
func (installer *Installer) Install(requested string) (string, error) {
	requested = strings.TrimPrefix(requested, "v")
	index, err := fetchIndex(installer.Client, installer.ToolsURL+"/editor/index.json")
	if err != nil {
		return "", err
	}
	entry, err := index.selectEntry(requested, installer.CliVersion, installer.Platform)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(installer.versionDir(entry.Version)); err == nil {
		if err := installer.setCurrent(entry.Version); err != nil {
			return "", err
		}
		installer.println("Mugon Editor %s is already installed and now active.", entry.Version)
		return entry.Version, nil
	}
	if err := installer.download(entry); err != nil {
		return "", err
	}
	if err := installer.setCurrent(entry.Version); err != nil {
		return "", err
	}
	installer.println("Installed Mugon Editor %s to %s", entry.Version, installer.versionDir(entry.Version))
	return entry.Version, nil
}

// Update installs the newest compatible version unless it is already current.
func (installer *Installer) Update() (string, error) {
	index, err := fetchIndex(installer.Client, installer.ToolsURL+"/editor/index.json")
	if err != nil {
		return "", err
	}
	entry, err := index.selectEntry("", installer.CliVersion, installer.Platform)
	if err != nil {
		return "", err
	}
	if installer.Current() == entry.Version {
		installer.println("Mugon Editor %s is already the newest compatible version.", entry.Version)
		return entry.Version, nil
	}
	return installer.Install(entry.Version)
}

// Uninstall removes one version, or all of them when version is empty. The active version falls back to the newest
// remaining one.
func (installer *Installer) Uninstall(version string, all bool) error {
	version = strings.TrimPrefix(version, "v")
	switch {
	case all:
		for _, installed := range installer.Installed() {
			if err := os.RemoveAll(installer.versionDir(installed)); err != nil {
				return err
			}
		}
		if err := os.Remove(filepath.Join(installer.Root, currentFileName)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		installer.println("Removed all installed editor versions.")
		return nil
	case version == "":
		version = installer.Current()
		if version == "" {
			return errors.New("no Mugon Editor is installed")
		}
	}
	if _, err := parseSemver(version); err != nil {
		return err
	}
	if _, err := os.Stat(installer.versionDir(version)); err != nil {
		return fmt.Errorf("mugon Editor %s is not installed", version)
	}
	if err := os.RemoveAll(installer.versionDir(version)); err != nil {
		return err
	}
	if installer.Current() == version {
		remaining := installer.Installed()
		if len(remaining) == 0 {
			if err := os.Remove(filepath.Join(installer.Root, currentFileName)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		} else if err := installer.setCurrent(remaining[len(remaining)-1]); err != nil {
			return err
		}
	}
	installer.println("Removed Mugon Editor %s.", version)
	return nil
}

func (installer *Installer) setCurrent(version string) error {
	if err := os.MkdirAll(installer.Root, 0o755); err != nil {
		return err
	}
	temporary := filepath.Join(installer.Root, currentFileName+".tmp")
	if err := os.WriteFile(temporary, []byte(version+"\n"), 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, filepath.Join(installer.Root, currentFileName))
}

func (installer *Installer) download(entry Entry) error {
	asset := entry.Assets[installer.Platform]
	installer.println("Downloading Mugon Editor %s (%s)...", entry.Version, installer.Platform)
	if err := os.MkdirAll(installer.Root, 0o755); err != nil {
		return err
	}
	archive, err := os.CreateTemp(installer.Root, "download-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = archive.Close()
		_ = os.Remove(archive.Name())
	}()

	response, err := installer.Client.Get(asset.URL)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", asset.URL, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: %s", asset.URL, response.Status)
	}
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(archive, hash), io.LimitReader(response.Body, maxArchiveBytes)); err != nil {
		return fmt.Errorf("downloading %s: %w", asset.URL, err)
	}
	if actual := hex.EncodeToString(hash.Sum(nil)); !strings.EqualFold(actual, asset.SHA256) {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", asset.URL, asset.SHA256, actual)
	}

	staging := filepath.Join(installer.Root, "."+entry.Version+".tmp")
	if err := os.RemoveAll(staging); err != nil {
		return err
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return err
	}
	if strings.HasSuffix(strings.ToLower(asset.URL), ".zip") {
		err = extractZip(archive.Name(), staging)
	} else {
		_, _ = archive.Seek(0, io.SeekStart)
		err = extractTarGz(archive, staging)
	}
	if err != nil {
		_ = os.RemoveAll(staging)
		return fmt.Errorf("unpacking the editor archive: %w", err)
	}
	if err := os.Rename(staging, installer.versionDir(entry.Version)); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	return nil
}

// targetPath maps an archive member below the archive's top-level folder to a path inside destination, rejecting
// anything that escapes it.
func targetPath(destination, name string) (string, bool, error) {
	clean := path.Clean(strings.ReplaceAll(name, "\\", "/"))
	if clean == "." || clean == archiveRootDir {
		return "", false, nil
	}
	clean = strings.TrimPrefix(clean, archiveRootDir+"/")
	if path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, ":") {
		return "", false, fmt.Errorf("unsafe path in archive: %s", name)
	}
	return filepath.Join(destination, filepath.FromSlash(clean)), true, nil
}

func extractTarGz(reader io.Reader, destination string) error {
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return err
	}
	defer func() { _ = gzipReader.Close() }()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		target, ok, err := targetPath(destination, header.Name)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := writeFile(target, tarReader, os.FileMode(header.Mode)&0o777|0o600); err != nil {
				return err
			}
		}
	}
}

func extractZip(archivePath, destination string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	for _, file := range reader.File {
		target, ok, err := targetPath(destination, file.Name)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		source, err := file.Open()
		if err != nil {
			return err
		}
		err = writeFile(target, source, 0o755)
		_ = source.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func writeFile(target string, source io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, io.LimitReader(source, maxArchiveBytes)); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
