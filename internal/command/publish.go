package command

import (
	"fmt"

	"github.com/mugon-net/cli/internal/model"
)

func ExecutePublishCommand(globalConfig model.GlobalConfig, projectConfig model.ProjectConfig) error {
	fmt.Println("Ran Publish Command")
	return nil
}
