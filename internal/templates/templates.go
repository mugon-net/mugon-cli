package templates

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/mugon-net/cli/internal/model"
)

//go:embed all:project
var projectTemplatesFS embed.FS

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

	return fs.WalkDir(projectTemplatesFS, "project", func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == "" {
			return nil
		}

		relPath, err := filepath.Rel("project", path)
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
