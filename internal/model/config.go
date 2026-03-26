package model

type GlobalConfig struct {
	Credentials map[string]string
}

type ProjectConfig struct {
	Id              string `toml:"id" validate:"required"`
	Version         string `toml:"version" validate:"required,semver"`
	JsSdkVersion    string `toml:"js-sdk-version" validate:"required,semver"`
	DistributionDir string `toml:"distribution-dir" validate:"required"`
	SourceDir       string `toml:"source-dir" validate:"required"`

	Commands []CommandConfig `toml:"commands"`
}

type CommandConfig struct {
	Name    string `toml:"name" validate:"required"`
	Command string `toml:"command" validate:"required"`
	Os      OsEnum `toml:"os" validate:"oneof-independent windows linux darwin"`
	Scope   string `toml:"scope"`
}

type OsEnum string

const (
	OsEnumIndependent OsEnum = "independent"
	OsEnumWindows     OsEnum = "windows"
	OsEnumLinux       OsEnum = "linux"
	OsEnumDarwin      OsEnum = "darwin"
)

type TemplateEnum string

const (
	TemplateEnumMinimal    = "minimal"
	TemplateEnumTypescript = "typescript"
	TemplateEnumBevy       = "bevy"
)

var TemplateEnums = []TemplateEnum{
	TemplateEnumMinimal,
	TemplateEnumTypescript,
	TemplateEnumBevy,
}
