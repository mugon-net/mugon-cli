package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/mugon-net/cli/internal/command"
	"github.com/mugon-net/cli/internal/model"
	"github.com/urfave/cli/v3"
)

type Config struct {
	GlobalConfig  model.GlobalConfig
	ProjectConfig *model.ProjectConfig
}

var config = Config{}

func main() {
	// commands:
	// - login
	//      Enter api key, authenticate & save credentials locally (maybe also roles? maybe later)
	//          Also needs options to directly pass credentials to be able to be used in ci/cd
	// - init
	//      Init a new game project in the cwd, creates a mugon.yaml file
	//          A bit like .firebaserc, includes output folder, configs, game id, js-sdk version, ...
	//          Also allow user to select templates: empty, typescript, bevy
	// - publish
	//      Publish current files in the output folder as a new version
	//          1. Creates the new (preliminary) version via the api
	//          Repeat until all files uploaded:
	//              2. Retrieves up to 10 presigned urls at a time to upload them
	//              3. Uploads the files using
	//          4. "Finish" the version creation, backend does a couple of checks, removes "preliminary" status
	//              Maybe copies the files to the serving bucket location? Should they still be seperated?
	//              Probably, so auto clean up can take care of half uploaded version files?

	root := &cli.Command{
		Name:  "mugon",
		Usage: "The mugon.net command line interface",
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			home, _ := os.UserHomeDir()
			globalPath := filepath.Join(home, ".mugon-cli.toml")

			var globalConfig model.GlobalConfig
			if _, err := toml.DecodeFile(globalPath, &globalConfig); err != nil {
				config.GlobalConfig = model.GlobalConfig{
					Credentials: make(map[string]string),
				}
				return ctx, nil
			}
			config.GlobalConfig = globalConfig

			return ctx, nil
		},
		Commands: []*cli.Command{
			{
				Name:  "init",
				Usage: "",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return command.ExecuteInitCommand(config.GlobalConfig)
				},
			},
			{
				Name:  "login",
				Usage: "",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return command.ExecuteLoginCommand(config.GlobalConfig)
				},
			},
			{
				Name:   "publish",
				Usage:  "",
				Before: ReadProjectConfig,
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return command.ExecutePublishCommand(config.GlobalConfig, *config.ProjectConfig)
				},
			},
			{
				Name:   "dev",
				Usage:  "",
				Before: ReadProjectConfig,
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return command.ExecuteDevCommand(config.GlobalConfig, *config.ProjectConfig)
				},
			},
		},
	}

	if err := root.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func ReadProjectConfig(ctx context.Context, c *cli.Command) (context.Context, error) {
	cwd, _ := os.Getwd()
	projectConfigPath := filepath.Join(cwd, "mugon.toml")

	var projectConfig model.ProjectConfig
	if _, err := toml.DecodeFile(projectConfigPath, &projectConfig); err != nil {
		return ctx, fmt.Errorf("No mugon.toml file found. Run 'mugon init' to initialize your project.")
	}
	config.ProjectConfig = &projectConfig

	return ctx, nil
}
