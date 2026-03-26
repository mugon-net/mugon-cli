package command

import (
	"context"
	"fmt"

	"github.com/mugon-net/cli/internal/model"
	"github.com/urfave/cli/v3"
)

func ExecuteDevCommand(ctx context.Context, c *cli.Command) error {
	_ = model.GetGlobalConfig(ctx)
	_ = model.GetProjectConfig(ctx)

	return fmt.Errorf("dev command not implemented yet")
}
