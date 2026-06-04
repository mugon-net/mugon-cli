package templates

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/mugon-net/cli/internal/model"
)

//go:embed all:project
var projectTemplatesFS embed.FS

//go:embed all:parentframe
var ParentframeFS embed.FS

func ParentframeFileSystem(projectConfig model.ProjectConfig, mainFramePort int, gameFramePort int) (http.FileSystem, error) {
	parentframeSubFS, err := fs.Sub(ParentframeFS, "parentframe")
	if err != nil {
		return nil, err
	}
	data := map[string]string{
		"ProjectId":              projectConfig.Id,
		"JsSdkVersion":           projectConfig.JsSdkVersion,
		"DistributionDir":        projectConfig.DistributionDir,
		"DefaultDistributionDir": model.DefaultDistributionDir,
		"SourceDir":              projectConfig.SourceDir,
		"DefaultSourceDir":       model.DefaultSourceDir,
		"MainFramePort":          fmt.Sprint(mainFramePort),
		"GameFramePort":          fmt.Sprint(gameFramePort),
	}
	return NewTemplateFS(parentframeSubFS, data), nil
}

func NewTemplateFS(fsys fs.FS, data map[string]string) http.FileSystem {
	return &templateFileSystem{fsys: fsys, data: data}
}

type templateFileSystem struct {
	fsys fs.FS
	data map[string]string
}

func (t *templateFileSystem) Open(name string) (http.File, error) {
	clean := strings.TrimPrefix(name, "/")
	if clean == "" {
		clean = "."
	}

	info, err := fs.Stat(t.fsys, clean)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return http.FS(t.fsys).Open(name)
	}

	raw, err := fs.ReadFile(t.fsys, clean)
	if err != nil {
		return nil, err
	}

	tmpl, err := template.New(name).Parse(string(raw))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, t.data); err != nil {
		return nil, err
	}

	processed := buf.Bytes()
	return &memFile{
		Reader: bytes.NewReader(processed),
		info:   memFileInfo{name: info.Name(), size: int64(len(processed))},
	}, nil
}

type memFile struct {
	*bytes.Reader
	info memFileInfo
}

func (m *memFile) Close() error                       { return nil }
func (m *memFile) Readdir(int) ([]fs.FileInfo, error) { return nil, nil }
func (m *memFile) Stat() (fs.FileInfo, error)         { return m.info, nil }

type memFileInfo struct {
	name string
	size int64
}

func (i memFileInfo) Name() string       { return i.name }
func (i memFileInfo) Size() int64        { return i.size }
func (i memFileInfo) Mode() fs.FileMode  { return 0444 }
func (i memFileInfo) ModTime() time.Time { return time.Time{} }
func (i memFileInfo) IsDir() bool        { return false }
func (i memFileInfo) Sys() any           { return nil }

func InstantiateProjectTemplate(
	id string,
	jsSdkVersion string,
	distributionDir string,
	sourceDir string,
	templateType model.TemplateEnum,
) error {
	data := map[string]string{
		"ProjectId":    id,
		"JsSdkVersion": jsSdkVersion,

		"DistributionDir":        distributionDir,
		"DefaultDistributionDir": model.DefaultDistributionDir,

		"SourceDir":        sourceDir,
		"DefaultSourceDir": model.DefaultSourceDir,

		"TemplateType": string(templateType),
	}

	folderPath := fmt.Sprintf("project/%s", string(templateType))
	return fs.WalkDir(projectTemplatesFS, folderPath, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == "" {
			return nil
		}

		relPath, err := filepath.Rel(folderPath, path)
		if err != nil {
			return err
		}

		targetPath := strings.Replace(relPath, ".template", "", 1)

		if dirEntry.IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
		} else {
			content, err := projectTemplatesFS.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read template file %s: %w", path, err)
			}

			processedContent := content
			if strings.HasSuffix(path, ".template") {
				tmpl, err := template.New(targetPath).Parse(string(content))
				if err == nil {
					var buf strings.Builder
					if err := tmpl.Execute(&buf, data); err == nil {
						re := regexp.MustCompile(`\n{3,}`)
						processedContent = []byte(re.ReplaceAllString(strings.ReplaceAll(buf.String(), "\r\n", "\n"), "\n\n"))
					}
				}
			}

			if err := os.WriteFile(targetPath, processedContent, 0644); err != nil {
				return fmt.Errorf("failed to write file %s: %w", targetPath, err)
			}
		}

		return nil
	})
}
