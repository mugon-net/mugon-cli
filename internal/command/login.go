package command

import (
	"context"
	"fmt"

	"github.com/mugon-net/cli/internal/model"
	"github.com/urfave/cli/v3"
)

func ExecuteLoginCommand(ctx context.Context, c *cli.Command) error {
	_ = model.GetGlobalConfig(ctx)

	// TODO prompt user for credentials interactively
	// Should be either a personal access token, or a game api key
	// PAT isnt implemented yet in the backend, so only game api key
	// Command isn't needed for first iteration, do it later

	return fmt.Errorf("Login command not implemented yet.")
}
