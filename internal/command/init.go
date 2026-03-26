package command

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/mugon-net/cli/internal/model"
	"github.com/mugon-net/cli/internal/templates"
	"github.com/urfave/cli/v3"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func ExecuteInitCommand(ctx context.Context, c *cli.Command) error {
	_, err := os.Stat("mugon.toml")
	if err == nil {
		return fmt.Errorf("'mugon.toml' already exists. Please run this command in a directory without an existing mugon project.")
	}

	// TODO Future: When PATs exist, prompt here if creating new game project or select existing one

	var (
		projectName      string
		selectedTemplate string
		jsSdkVersion     = model.DefaultJsSdkVersion
		distributionDir  = model.DefaultDistributionDir
		sourceDir        = model.DefaultSourceDir
	)

	templateOptions := make([]huh.Option[string], len(model.TemplateEnums))
	for i, v := range model.TemplateEnums {
		templateOptions[i] = huh.NewOption(cases.Title(language.English, cases.Compact).String(string(v)), string(v))
	}

	err = huh.NewInput().
		Title("Project name").
		Value(&projectName).
		Validate(func(str string) error {
			_, err := getProjectId(projectName)
			return err
		}).
		Run()
	if err != nil {
		return err
	}

	err = huh.NewSelect[string]().
		Title("Project template").
		Options(templateOptions...).
		Value(&selectedTemplate).
		Run()
	if err != nil {
		return err
	}

	if selectedTemplate == model.TemplateEnumMinimal {
		err = huh.NewInput().
			Title("Distribution folder").
			Value(&distributionDir).
			Run()
		if err != nil {
			return err
		}
		err = huh.NewInput().
			Title("Source folder").
			Value(&sourceDir).
			Run()
		if err != nil {
			return err
		}
	}

	projectId, _ := getProjectId(projectName)

	return templates.InstantiateProjectTemplate(projectId, jsSdkVersion, distributionDir, sourceDir, model.TemplateEnum(selectedTemplate))
}

func getProjectId(projectName string) (string, error) {
	reg := regexp.MustCompile(`[^a-zA-Z0-9-]+`)
	projectId := reg.ReplaceAllString(strings.ReplaceAll(strings.TrimSpace(strings.ToLower(projectName)), " ", "-"), "")
	if strings.HasPrefix(projectId, "-") || strings.HasSuffix(projectId, "-") {
		return "", fmt.Errorf("Invalid project name.")
	}
	return projectId, nil
}
