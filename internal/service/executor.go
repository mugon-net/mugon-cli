package service

import (
	"os"
	"os/exec"
	"runtime"

	"github.com/mugon-net/cli/internal/model"
)

func ExecuteProjectCommand(commandConfig model.CommandConfig, globalConfig model.GlobalConfig, projectConfig model.ProjectConfig) error {

	var shell string
	var shellParameters []string

	switch runtime.GOOS {
	case "windows":
		shell = "cmd"
		shellParameters = []string{"/d", "/s", "/c"}
	default:
		shell = "sh"
		shellParameters = []string{"-c"}
	}

	executedCommand := exec.Command(shell, append(shellParameters, commandConfig.Command)...)

	executedCommand.Stdout = os.Stdout
	executedCommand.Stderr = os.Stderr
	// If interactive command is used in command
	executedCommand.Stdin = os.Stdin
	err := executedCommand.Run()

	return err
}
