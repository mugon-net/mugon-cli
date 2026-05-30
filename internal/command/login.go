package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/mugon-net/cli/internal/model"
	"github.com/mugon-net/cli/internal/service"
	"github.com/urfave/cli/v3"
)

func ExecuteLoginCommand(ctx context.Context, c *cli.Command) error {
	globalConfig := model.GetGlobalConfig(ctx)

	projectId := c.String("project-id")
	apiKey := c.String("api-key")

	if projectId == "" {
		projectId = model.TryReadProjectId()
	}

	if projectId == "" || apiKey == "" {
		if err := runLoginPrompts(&projectId, &apiKey); err != nil {
			return err
		}
	}

	projectId = strings.TrimSpace(projectId)
	apiKey = strings.TrimSpace(apiKey)

	api, err := service.InitApiWithKey(*globalConfig, apiKey)
	if err != nil {
		return err
	}

	if err := api.ValidateApiKey(ctx, projectId); err != nil {
		return err
	}

	if globalConfig.Credentials == nil {
		globalConfig.Credentials = make(map[string]string)
	}
	globalConfig.Credentials[projectId] = apiKey

	if err := model.WriteGlobalConfig(globalConfig); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Printf("Logged in for project '%s'\n", projectId)
	return nil
}

func runLoginPrompts(projectId *string, apiKey *string) error {
	if *projectId == "" {
		err := huh.NewInput().
			Title("Project ID").
			Description("The project ID from mugon.toml or the mugon.net dashboard").
			Value(projectId).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("project ID cannot be empty")
				}
				return nil
			}).
			Run()
		if err != nil {
			return err
		}
	}

	if *apiKey == "" {
		err := huh.NewInput().
			Title("API Key").
			EchoMode(huh.EchoModePassword).
			Value(apiKey).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("API key cannot be empty")
				}
				return nil
			}).
			Run()
		if err != nil {
			return err
		}
	}

	return nil
}
