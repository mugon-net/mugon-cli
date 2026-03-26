package model

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
