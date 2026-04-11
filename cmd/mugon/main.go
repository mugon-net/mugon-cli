package main

import (
	"context"
	"os"

	"github.com/mugon-net/cli/internal/command"
	"github.com/mugon-net/cli/internal/logging"
	"github.com/mugon-net/cli/internal/model"
	"github.com/urfave/cli/v3"
)

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
		Name:   "mugon",
		Usage:  "The mugon.net command line interface",
		Before: model.ReadGlobalConfig,
		Commands: []*cli.Command{
			{
				Name:   "init",
				Usage:  "Initializes a new project in the current directory.",
				Action: command.ExecuteInitCommand,
			},
			/*{
				Name:   "login",
				Usage:  "Saves credentials for a project globally [Not implemented yet]",
				Action: command.ExecuteLoginCommand,
			},*/
			/*{
				Name:   "logout",
				Usage:  "Removes credentials for a project [Not implemented yet]",
				Action: command.ExecuteLogoutCommand,
			},*/
			/*{
				Name:   "dev",
				Usage:  "Runs the development suite for the project in the current directory [Not implemented yet]",
				Before: model.ReadProjectConfig,
				Action: command.ExecuteDevCommand,
			},*/
			{
				Name:   "publish",
				Usage:  "Publishes the project in the current directory as a new version on mugon.net",
				Before: model.ReadProjectConfig,
				Action: command.ExecutePublishCommand,
			},
			{
				Name:   "run",
				Usage:  "Runs commands configured in the project file",
				Before: model.ReadProjectConfig,
				Action: command.ExecuteRunCommand,
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name:      "command",
						UsageText: "The command name as specified in the mugon.toml file",
						Config: cli.StringConfig{
							TrimSpace: true,
						},
					},
				},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "scope",
						Value: model.DefaultCommandScope,
						Usage: "The scope the command should be run in",
					},
				},
			},
		},
	}

	translator := logging.GetValidatorTranslator()
	ctx := context.WithValue(context.Background(), model.ContextValueEnumValidatorTranslator, translator)
	if err := root.Run(ctx, os.Args); err != nil {
		logging.PrintUserFacingErrorMessage(err, translator)
		os.Exit(1)
	}
}
