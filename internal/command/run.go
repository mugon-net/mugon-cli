package command

import (
	"context"
	"strings"

	"github.com/mugon-net/cli/internal/model"
	"github.com/mugon-net/cli/internal/service"
	"github.com/urfave/cli/v3"
)

func ExecuteRunCommand(ctx context.Context, c *cli.Command) error {
	globalConfig := model.GetGlobalConfig(ctx)
	projectConfig := model.GetProjectConfig(ctx)

	commandArg := c.StringArg("command")
	scopeArg := c.String("scope")
	if len(strings.TrimSpace(scopeArg)) == 0 {
		scopeArg = model.DefaultCommandScope
	}

	commandConfig, err := projectConfig.GetCommand(commandArg, scopeArg)
	if err != nil {
		return err
	}

	err = service.ExecuteProjectCommand(commandConfig, *globalConfig, *projectConfig)
	if err != nil {
		return err
	}

	return nil
}
