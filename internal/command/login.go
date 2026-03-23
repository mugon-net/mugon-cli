package command

import (
	"fmt"

	"github.com/mugon-net/cli/internal/model"
)

func ExecuteLoginCommand(globalConfig model.GlobalConfig) error {
	// TODO prompt user for credentials interactively
	// Should be either a personal access token, or a game api key
	// PAT isnt implemented yet in the backend, so only game api key
	// Command isn't needed for first iteration, do it later
	return fmt.Errorf("Login command not implemented yet.")
}
