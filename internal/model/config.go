package model

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/mugon-net/cli/internal/logging"
	"github.com/urfave/cli/v3"
)

type GlobalConfig struct {
	Credentials            map[string]string `toml:"credentials" validate:"dive"`
	MugonNetApiUrlOverride string            `toml:"mugon-net-api-url-override"`
}

type ProjectConfig struct {
	Id              string `toml:"id" validate:"required"`
	Version         string `toml:"version" validate:"required,semver"`
	JsSdkVersion    string `toml:"js-sdk-version" validate:"required,semver"`
	DistributionDir string `toml:"distribution-dir" validate:"required"`
	SourceDir       string `toml:"source-dir" validate:"required"`

	Commands []CommandConfig `toml:"command" validate:"dive"`
}

type CommandConfig struct {
	Name    string `toml:"name" validate:"required"`
	Command string `toml:"command" validate:"required"`
	Os      OsEnum `toml:"os" validate:"required,oneof=independent windows linux darwin"`
	Scope   string `toml:"scope" validate:"required"`
}

func ReadGlobalConfig(ctx context.Context, c *cli.Command) (context.Context, error) {
	home, _ := os.UserHomeDir()
	globalPath := filepath.Join(home, ".mugon-cli.toml")

	var globalConfig GlobalConfig
	if _, err := toml.DecodeFile(globalPath, &globalConfig); err != nil {
		return context.WithValue(ctx, ContextValueEnumGlobalConfig, &GlobalConfig{
			Credentials: make(map[string]string),
		}), nil
	}
	return context.WithValue(ctx, ContextValueEnumGlobalConfig, &globalConfig), nil
}

func ReadProjectConfig(ctx context.Context, c *cli.Command) (context.Context, error) {
	cwd, _ := os.Getwd()
	projectConfigPath := filepath.Join(cwd, "mugon.toml")

	projectConfig := ProjectConfig{
		DistributionDir: DefaultDistributionDir,
		SourceDir:       DefaultSourceDir,
		Commands:        []CommandConfig{},
	}

	if _, err := toml.DecodeFile(projectConfigPath, &projectConfig); err != nil {
		return ctx, fmt.Errorf("no mugon.toml file found. Run 'mugon init' to initialize your project")
	}

	for i, v := range projectConfig.Commands {
		if strings.TrimSpace(v.Scope) == "" {
			v.Scope = DefaultCommandScope
		}

		if strings.TrimSpace(string(v.Os)) == "" {
			v.Os = DefaultCommandOs
		}
		projectConfig.Commands[i] = v
	}

	validate := validator.New(validator.WithRequiredStructEnabled())
	logging.SetupValidatorLogging(validate, GetValidatorTranslator(ctx))

	err := validate.Struct(projectConfig)
	if err != nil {
		return ctx, err
	}

	return context.WithValue(ctx, ContextValueEnumProjectConfig, &projectConfig), nil
}

func (projectConfig *ProjectConfig) GetCommand(name string, scope string) (CommandConfig, error) {
	os := runtime.GOOS
	for _, cmd := range projectConfig.Commands {
		if cmd.Name == name && cmd.Scope == scope && (string(cmd.Os) == os || cmd.Os == OsEnumIndependent) {
			return cmd, nil
		}
	}

	return CommandConfig{}, CommandNotFoundRunError{}
}

func GetProjectConfig(ctx context.Context) *ProjectConfig {
	return ctx.Value(ContextValueEnumProjectConfig).(*ProjectConfig)
}

func GetGlobalConfig(ctx context.Context) *GlobalConfig {
	return ctx.Value(ContextValueEnumGlobalConfig).(*GlobalConfig)
}

func GetValidatorTranslator(ctx context.Context) ut.Translator {
	return ctx.Value(ContextValueEnumValidatorTranslator).(ut.Translator)
}

func GetApiKey(globalConfig GlobalConfig, projectConfig ProjectConfig) (string, error) {
	projectId := projectConfig.Id
	apiKey, ok := globalConfig.Credentials[projectId]
	apiKeyFromEnv, okFromEnv := os.LookupEnv("MUGON_PROJECT_API_KEY")
	if okFromEnv {
		apiKey = apiKeyFromEnv
	} else if !ok {
		return "", NoProjectApiKeyFound{}
	}
	return apiKey, nil
}
