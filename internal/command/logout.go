package command

import (
	"context"
	"fmt"

	"github.com/mugon-net/cli/internal/model"
	"github.com/urfave/cli/v3"
)

func ExecuteLogoutCommand(ctx context.Context, c *cli.Command) error {
	_ = model.GetGlobalConfig(ctx)

	return fmt.Errorf("Logout command not implemented yet.")
}
