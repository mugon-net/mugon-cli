package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/mugon-net/cli/internal/model"
)

func instantiate(t *testing.T, templateType model.TemplateEnum, distributionDir string) string {
	t.Helper()
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := InstantiateProjectTemplate("probe", model.DefaultJsSdkVersion, distributionDir, "src", templateType); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestTemplatePathsAreRendered(t *testing.T) {
	for _, templateType := range model.TemplateEnums {
		dir := instantiate(t, templateType, "out-dir")
		_ = filepath.Walk(dir, func(path string, _ os.FileInfo, _ error) error {
			if strings.Contains(path, "{{") {
				t.Errorf("%s: unrendered path %s", templateType, path)
			}
			return nil
		})
	}
}

func TestShellTemplatesShipTheSdkInTheDistributionFolder(t *testing.T) {
	for _, templateType := range []model.TemplateEnum{model.TemplateEnumMinimal} {
		dir := instantiate(t, templateType, "dist")
		for _, file := range []string{"index.html", "mugon.iife.js"} {
			if _, err := os.Stat(filepath.Join(dir, "dist", file)); err != nil {
				t.Errorf("%s: missing dist/%s: %v", templateType, file, err)
			}
		}
		html, err := os.ReadFile(filepath.Join(dir, "dist", "index.html"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Index(string(html), "mugon.iife.js") > strings.Index(string(html), "<script type=\"module\"") && strings.Contains(string(html), "<script type=\"module\"") {
			t.Errorf("%s: the SDK must be the first script in index.html", templateType)
		}
	}
}

func TestBundlerTemplatesDependOnTheSdkAndShipNothingInTheDistributionFolder(t *testing.T) {
	for _, templateType := range []model.TemplateEnum{model.TemplateEnumTypescript, model.TemplateEnumBevy} {
		dir := instantiate(t, templateType, "dist")
		if _, err := os.Stat(filepath.Join(dir, "dist")); err == nil {
			t.Errorf("%s: the template must not ship files in the distribution folder", templateType)
		}
		manifest, err := os.ReadFile(filepath.Join(dir, "package.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(manifest), "\"@mugon/sdk\": \""+model.DefaultJsSdkVersion+"\"") {
			t.Errorf("%s: package.json must depend on @mugon/sdk %s", templateType, model.DefaultJsSdkVersion)
		}
	}
}

func TestBevyTemplateHasADevAndAReleaseBuildCommand(t *testing.T) {
	dir := instantiate(t, model.TemplateEnumBevy, "dist")
	var config model.ProjectConfig
	if _, err := toml.DecodeFile(filepath.Join(dir, "mugon.toml"), &config); err != nil {
		t.Fatal(err)
	}
	scopes := map[string]string{}
	for _, command := range config.Commands {
		if command.Name == "build" {
			scopes[command.Scope] = command.Command
		}
	}
	if !strings.Contains(scopes["dev"], "target/wasm32-unknown-unknown/release/game.wasm") {
		t.Errorf("dev build command must use the release profile: %q", scopes["dev"])
	}
	if !strings.Contains(scopes[""], "--profile release-lto") || !strings.Contains(scopes[""], "release-lto/game.wasm") {
		t.Errorf("default build command must use the release-lto profile: %q", scopes[""])
	}
}
