package model

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/mugon-net/cli/internal/oapi"
)

func decodeSettings(t *testing.T, tomlSource string) []SettingConfig {
	t.Helper()
	var config ProjectConfig
	if _, err := toml.Decode(tomlSource, &config); err != nil {
		t.Fatalf("failed to decode toml: %v", err)
	}
	return config.Settings
}

func TestGetSettingDTOs_AllTypes(t *testing.T) {
	settings := decodeSettings(t, `
[[setting]]
key = "friendly-fire"
label = "Friendly fire"
tab = "Gameplay"
type = "boolean"
default = false

[[setting]]
key = "round-time"
type = "number"
min = 30
max = 600
step = 30
default = 120
ui = "slider"

[[setting]]
key = "map"
type = "select"
options = ["desert", "forest"]
default = "desert"

[[setting]]
key = "modifiers"
type = "select"
multiple = true
options = [{ value = "fast", label = "Fast rounds" }, { value = "low_gravity", label = "Low gravity" }]
default = ["fast", "low_gravity"]

[[setting]]
key = "server.name"
type = "text"
default = "My server"
max-length = 32
`)

	dtos, err := GetSettingDTOs(settings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dtos) != 5 {
		t.Fatalf("converted %d settings, want 5", len(dtos))
	}

	if dtos[0].Bool == nil || dtos[0].Bool.DefaultValue {
		t.Errorf("boolean payload = %+v", dtos[0].Bool)
	}
	if dtos[0].Label != "Friendly fire" || dtos[0].Tab != "Gameplay" {
		t.Errorf("label/tab = %q/%q", dtos[0].Label, dtos[0].Tab)
	}

	if dtos[1].Number == nil || dtos[1].Number.DefaultValue != 120 || dtos[1].Number.Step != 30 {
		t.Errorf("number payload = %+v", dtos[1].Number)
	}
	if dtos[1].Ui == nil || *dtos[1].Ui != oapi.GameVersionSettingUiEnumSLIDER {
		t.Errorf("ui = %+v", dtos[1].Ui)
	}
	if dtos[1].Label != "round-time" {
		t.Errorf("label = %q, want the key as fallback", dtos[1].Label)
	}

	if dtos[2].Select == nil || len(dtos[2].Select.Options) != 2 {
		t.Fatalf("select payload = %+v", dtos[2].Select)
	}
	if dtos[2].Select.Options[0].Label != "desert" {
		t.Errorf("bare string option label = %q, want the value", dtos[2].Select.Options[0].Label)
	}
	if len(dtos[2].Select.DefaultValue) != 1 || dtos[2].Select.DefaultValue[0] != "desert" {
		t.Errorf("single select default = %+v", dtos[2].Select.DefaultValue)
	}

	if dtos[3].Select == nil || !dtos[3].Select.Multiple {
		t.Fatalf("multi select payload = %+v", dtos[3].Select)
	}
	if dtos[3].Select.Options[0].Label != "Fast rounds" {
		t.Errorf("table option label = %q", dtos[3].Select.Options[0].Label)
	}
	if len(dtos[3].Select.DefaultValue) != 2 {
		t.Errorf("multi select default = %+v", dtos[3].Select.DefaultValue)
	}

	if dtos[4].Text == nil || dtos[4].Text.DefaultValue != "My server" {
		t.Fatalf("text payload = %+v", dtos[4].Text)
	}
	if dtos[4].Text.MaxLength == nil || *dtos[4].Text.MaxLength != 32 {
		t.Errorf("max-length = %+v", dtos[4].Text.MaxLength)
	}
}

func TestGetSettingDTOs_NumberDefaultsStepToOne(t *testing.T) {
	settings := decodeSettings(t, `
[[setting]]
key = "players"
type = "number"
min = 1
max = 8
default = 4
`)

	dtos, err := GetSettingDTOs(settings)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dtos[0].Number.Step != 1 {
		t.Errorf("step = %v, want 1", dtos[0].Number.Step)
	}
}

func TestGetSettingDTOs_Rejects(t *testing.T) {
	tests := []struct {
		name string
		toml string
	}{
		{
			name: "reserved namespace",
			toml: "[[setting]]\nkey = \"mugon.serverid\"\ntype = \"boolean\"\ndefault = true",
		},
		{
			name: "unknown type",
			toml: "[[setting]]\nkey = \"k\"\ntype = \"colour\"\ndefault = \"red\"",
		},
		{
			name: "unknown ui",
			toml: "[[setting]]\nkey = \"k\"\ntype = \"boolean\"\nui = \"dial\"\ndefault = true",
		},
		{
			name: "default type mismatch",
			toml: "[[setting]]\nkey = \"k\"\ntype = \"boolean\"\ndefault = 3",
		},
		{
			name: "number without bounds",
			toml: "[[setting]]\nkey = \"k\"\ntype = \"number\"\ndefault = 3",
		},
		{
			name: "select without options",
			toml: "[[setting]]\nkey = \"k\"\ntype = \"select\"\ndefault = \"a\"",
		},
		{
			name: "select default not an option",
			toml: "[[setting]]\nkey = \"k\"\ntype = \"select\"\noptions = [\"a\"]\ndefault = \"b\"",
		},
		{
			name: "single select with two defaults",
			toml: "[[setting]]\nkey = \"k\"\ntype = \"select\"\noptions = [\"a\", \"b\"]\ndefault = [\"a\", \"b\"]",
		},
		{
			name: "duplicate key",
			toml: "[[setting]]\nkey = \"k\"\ntype = \"boolean\"\ndefault = true\n\n[[setting]]\nkey = \"k\"\ntype = \"text\"\ndefault = \"x\"",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := GetSettingDTOs(decodeSettings(t, test.toml)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
