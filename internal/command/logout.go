package command

import (
	"context"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/mugon-net/cli/internal/model"
	"github.com/urfave/cli/v3"
)

func ExecuteLogoutCommand(ctx context.Context, c *cli.Command) error {
	globalConfig := model.GetGlobalConfig(ctx)

	if len(globalConfig.Credentials) == 0 {
		fmt.Println("No credentials stored.")
		return nil
	}

	projectId := c.String("project-id")

	if projectId == "" {
		projectId = model.TryReadProjectId()
	}

	if projectId == "" {
		if len(globalConfig.Credentials) == 1 {
			for id := range globalConfig.Credentials {
				projectId = id
			}
		} else {
			options := make([]huh.Option[string], 0, len(globalConfig.Credentials))
			for id := range globalConfig.Credentials {
				options = append(options, huh.NewOption(id, id))
			}
			err := huh.NewSelect[string]().
				Title("Select project to log out from").
				Options(options...).
				Value(&projectId).
				Run()
			if err != nil {
				return err
			}
		}
	}

	if _, ok := globalConfig.Credentials[projectId]; !ok {
		return fmt.Errorf("no credentials stored for project '%s'", projectId)
	}

	delete(globalConfig.Credentials, projectId)

	if err := model.WriteGlobalConfig(globalConfig); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Printf("Logged out from project '%s'\n", projectId)
	return nil
}
