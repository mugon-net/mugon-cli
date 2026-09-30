package service

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The literal is embedded by @mugon/sdk's version.ts; keep in sync.
var sdkMarkerPattern = regexp.MustCompile(`mugon-sdk-version:(\d+\.\d+\.\d+)`)

var sdkScannedExtensions = map[string]bool{".js": true, ".mjs": true, ".cjs": true, ".html": true}

// FindSdkVersions returns the distinct SDK versions embedded in the text
// (js/html) files below distributionDir.
func FindSdkVersions(distributionDir string) ([]string, error) {
	found := make(map[string]bool)
	err := filepath.WalkDir(distributionDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !sdkScannedExtensions[strings.ToLower(filepath.Ext(path))] {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range sdkMarkerPattern.FindAllSubmatch(content, -1) {
			found[string(match[1])] = true
		}
		return nil
	})
	versions := make([]string, 0, len(found))
	for version := range found {
		versions = append(versions, version)
	}
	return versions, err
}

// SdkLoadedFirst reports whether the first <script> of index.html is inline
// SDK code or a file that contains it.
func SdkLoadedFirst(distributionDir string) bool {
	html, err := os.ReadFile(filepath.Join(distributionDir, "index.html"))
	if err != nil {
		return false
	}
	tag := firstScriptPattern.FindSubmatch(html)
	if tag == nil {
		return false
	}
	if sdkMarkerPattern.Match(tag[2]) {
		return true
	}
	src := scriptSrcPattern.FindSubmatch(tag[1])
	if src == nil {
		return false
	}
	reference := strings.TrimPrefix(string(src[1]), "/")
	if strings.Contains(reference, "://") || strings.HasPrefix(reference, "//") {
		return false
	}
	content, err := os.ReadFile(filepath.Join(distributionDir, filepath.FromSlash(reference)))
	if err != nil {
		return false
	}
	return sdkMarkerPattern.Match(content)
}

var (
	firstScriptPattern = regexp.MustCompile(`(?is)<script([^>]*)>(.*?)</script>`)
	scriptSrcPattern   = regexp.MustCompile(`(?i)\bsrc\s*=\s*["']([^"']+)["']`)
)
