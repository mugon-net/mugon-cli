package editorinstall

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
)

// Index is the editor index published at <tools host>/editor/index.json.
type Index struct {
	Versions []Entry `json:"versions"`
}

type Entry struct {
	Version string `json:"version"`
	// RequiresCli is a space-separated range such as ">=0.5.0 <1.0.0"; empty means any CLI.
	RequiresCli string           `json:"requiresCli"`
	Assets      map[string]Asset `json:"assets"`
}

type Asset struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// PlatformKey is the key of the asset for the given Go GOOS/GOARCH, e.g. "linux-amd64".
func PlatformKey(goos, goarch string) string {
	return goos + "-" + goarch
}

func CurrentPlatformKey() string {
	return PlatformKey(runtime.GOOS, runtime.GOARCH)
}

func fetchIndex(client *http.Client, url string) (Index, error) {
	response, err := client.Get(url)
	if err != nil {
		return Index{}, fmt.Errorf("fetching the editor index: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return Index{}, fmt.Errorf("fetching the editor index %s: %s", url, response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return Index{}, fmt.Errorf("reading the editor index: %w", err)
	}
	var index Index
	if err := json.Unmarshal(body, &index); err != nil {
		return Index{}, fmt.Errorf("parsing the editor index: %w", err)
	}
	return index, nil
}

// selectEntry returns the requested version, or the newest one that supports cliVersion and has an asset for platform.
func (index Index) selectEntry(requested, cliVersion, platform string) (Entry, error) {
	var best Entry
	var bestVersion semver
	found := false
	for _, entry := range index.Versions {
		if _, ok := entry.Assets[platform]; !ok {
			continue
		}
		version, err := parseSemver(entry.Version)
		if err != nil {
			continue
		}
		if requested != "" {
			if version.compare(mustParse(requested)) == 0 {
				return entry, nil
			}
			continue
		}
		compatible, err := satisfies(cliVersion, entry.RequiresCli)
		if err != nil || !compatible {
			continue
		}
		if !found || version.compare(bestVersion) > 0 {
			best, bestVersion, found = entry, version, true
		}
	}
	switch {
	case requested != "":
		return Entry{}, fmt.Errorf("editor version %s is not available for %s", requested, platform)
	case !found:
		return Entry{}, fmt.Errorf("no editor release for %s is compatible with this CLI (%s)", platform, cliVersion)
	}
	return best, nil
}

func mustParse(text string) semver {
	version, err := parseSemver(text)
	if err != nil {
		return semver{-1, -1, -1}
	}
	return version
}
