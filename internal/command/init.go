package command

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/mugon-net/cli/internal/model"
)

func ExecuteInitCommand(globalConfig model.GlobalConfig) error {

	_, err := os.Stat("mugon.toml")
	if err == nil {
		return fmt.Errorf("'mugon.toml' already exists. Please run again in a directory without existing mugon game project.")
	}

	// TODO Future: When PATs exist, prompt here if creating new game project or select existing one

	gameNamePrompt := promptui.Prompt{
		Label: "Enter game name",
		Validate: func(value string) error {
			_, err := getGameId(value)
			return err
		},
	}
	gameName, err := gameNamePrompt.Run()
	if err != nil {
		return err
	}
	gameId, err := getGameId(gameName)
	if err != nil {
		return err
	}

	fmt.Printf("Game id: '%s'\n", gameId)

	templatePrompt := promptui.Select{
		Label: "What kind of project template do you want to use?",
		Items: []string{
			"Bevy", "Javascript", "Empty",
		},
	}
	_, selectedTemplate, err := templatePrompt.Run()
	if err != nil {
		return err
	}
	fmt.Printf("TODO: '%s'\n", selectedTemplate)

	// TODO
	return nil
}

func getGameId(gameName string) (string, error) {
	reg := regexp.MustCompile(`[^a-zA-Z0-9-]+`)
	gameId := reg.ReplaceAllString(strings.ReplaceAll(strings.TrimSpace(strings.ToLower(gameName)), " ", "-"), "")
	if strings.HasPrefix(gameId, "-") || strings.HasSuffix(gameId, "-") {
		return "", fmt.Errorf("Invalid game name.")
	}
	return gameId, nil
}
