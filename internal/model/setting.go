package model

import (
	"fmt"
	"strings"

	"github.com/mugon-net/cli/internal/oapi"
)

type SettingTypeEnum string

const (
	SettingTypeEnumBoolean SettingTypeEnum = "boolean"
	SettingTypeEnumNumber  SettingTypeEnum = "number"
	SettingTypeEnumSelect  SettingTypeEnum = "select"
	SettingTypeEnumText    SettingTypeEnum = "text"
)

type SettingUiEnum string

const (
	SettingUiEnumSwitch           SettingUiEnum = "switch"
	SettingUiEnumCheckbox         SettingUiEnum = "checkbox"
	SettingUiEnumSlider           SettingUiEnum = "slider"
	SettingUiEnumNumberInput      SettingUiEnum = "number-input"
	SettingUiEnumDropdown         SettingUiEnum = "dropdown"
	SettingUiEnumSegmentedControl SettingUiEnum = "segmented-control"
	SettingUiEnumRadioCards       SettingUiEnum = "radio-cards"
	SettingUiEnumTextInput        SettingUiEnum = "text-input"
	SettingUiEnumTextarea         SettingUiEnum = "textarea"
)

var settingUiDtoEnums = map[SettingUiEnum]oapi.GameVersionSettingUiEnum{
	SettingUiEnumSwitch:           oapi.GameVersionSettingUiEnumSWITCH,
	SettingUiEnumCheckbox:         oapi.GameVersionSettingUiEnumCHECKBOX,
	SettingUiEnumSlider:           oapi.GameVersionSettingUiEnumSLIDER,
	SettingUiEnumNumberInput:      oapi.GameVersionSettingUiEnumNUMBERINPUT,
	SettingUiEnumDropdown:         oapi.GameVersionSettingUiEnumDROPDOWN,
	SettingUiEnumSegmentedControl: oapi.GameVersionSettingUiEnumSEGMENTEDCONTROL,
	SettingUiEnumRadioCards:       oapi.GameVersionSettingUiEnumRADIOCARDS,
	SettingUiEnumTextInput:        oapi.GameVersionSettingUiEnumTEXTINPUT,
	SettingUiEnumTextarea:         oapi.GameVersionSettingUiEnumTEXTAREA,
}

var settingTypeDtoEnums = map[SettingTypeEnum]oapi.GameVersionSettingTypeEnum{
	SettingTypeEnumBoolean: oapi.GameVersionSettingTypeEnumBOOLEAN,
	SettingTypeEnumNumber:  oapi.GameVersionSettingTypeEnumNUMBER,
	SettingTypeEnumSelect:  oapi.GameVersionSettingTypeEnumSELECT,
	SettingTypeEnumText:    oapi.GameVersionSettingTypeEnumTEXT,
}

// The namespace the mugon SDK itself occupies in the settings a game receives
// (mugon.networkmode, mugon.serverid, mugon.clientid).
const reservedSettingKeyPrefix = "mugon."

// SettingConfig is one `[[setting]]` entry in mugon.toml. Because TOML values
// are polymorphic, `default` and `options` are decoded loosely and resolved
// against `type` by ToDTO.
type SettingConfig struct {
	Key         string          `toml:"key" validate:"required"`
	Label       string          `toml:"label"`
	Description string          `toml:"description"`
	Tab         string          `toml:"tab"`
	Type        SettingTypeEnum `toml:"type" validate:"required,oneof=boolean number select text"`
	Ui          SettingUiEnum   `toml:"ui" validate:"omitempty,oneof=switch checkbox slider number-input dropdown segmented-control radio-cards text-input textarea"`

	Default any `toml:"default"`

	Min  *float64 `toml:"min"`
	Max  *float64 `toml:"max"`
	Step *float64 `toml:"step"`

	Multiple bool  `toml:"multiple"`
	Options  []any `toml:"options"`

	MaxLength *int32 `toml:"max-length"`

	// EnabledWhen is a CEL expression the initiator must satisfy before this
	// setting becomes editable; the backend validates it when publishing.
	EnabledWhen string `toml:"enabled-when"`
}

// ToDTO turns the loose TOML shape into the request payload. It reports the
// same problems the backend would so publishing fails before anything is
// uploaded; the backend stays the authority and re-validates.
func (setting SettingConfig) ToDTO() (oapi.GameVersionSettingsItemDTO, error) {
	key := strings.TrimSpace(setting.Key)
	if strings.HasPrefix(key, reservedSettingKeyPrefix) {
		return oapi.GameVersionSettingsItemDTO{}, fmt.Errorf("setting '%s' uses the reserved '%s' key namespace", key, reservedSettingKeyPrefix)
	}

	settingType, ok := settingTypeDtoEnums[setting.Type]
	if !ok {
		return oapi.GameVersionSettingsItemDTO{}, fmt.Errorf("setting '%s' has unknown type '%s'", key, setting.Type)
	}

	label := strings.TrimSpace(setting.Label)
	if label == "" {
		label = key
	}

	dto := oapi.GameVersionSettingsItemDTO{
		Key:         key,
		Label:       label,
		Description: strings.TrimSpace(setting.Description),
		Tab:         strings.TrimSpace(setting.Tab),
		Type:        settingType,
	}

	if enabledWhen := strings.TrimSpace(setting.EnabledWhen); enabledWhen != "" {
		dto.EnabledWhen = &enabledWhen
	}

	if setting.Ui != "" {
		ui, ok := settingUiDtoEnums[setting.Ui]
		if !ok {
			return oapi.GameVersionSettingsItemDTO{}, fmt.Errorf("setting '%s' has unknown ui '%s'", key, setting.Ui)
		}
		dto.Ui = &ui
	}

	switch setting.Type {
	case SettingTypeEnumBoolean:
		payload, err := setting.booleanDTO(key)
		if err != nil {
			return oapi.GameVersionSettingsItemDTO{}, err
		}
		dto.Bool = payload
	case SettingTypeEnumNumber:
		payload, err := setting.numberDTO(key)
		if err != nil {
			return oapi.GameVersionSettingsItemDTO{}, err
		}
		dto.Number = payload
	case SettingTypeEnumSelect:
		payload, err := setting.selectDTO(key)
		if err != nil {
			return oapi.GameVersionSettingsItemDTO{}, err
		}
		dto.Select = payload
	case SettingTypeEnumText:
		payload, err := setting.textDTO(key)
		if err != nil {
			return oapi.GameVersionSettingsItemDTO{}, err
		}
		dto.Text = payload
	}

	return dto, nil
}

func (setting SettingConfig) booleanDTO(key string) (*oapi.GameVersionSettingBoolDTO, error) {
	defaultValue, ok := setting.Default.(bool)
	if !ok {
		return nil, fmt.Errorf("setting '%s': default must be a boolean", key)
	}
	return &oapi.GameVersionSettingBoolDTO{DefaultValue: defaultValue}, nil
}

func (setting SettingConfig) numberDTO(key string) (*oapi.GameVersionSettingNumberDTO, error) {
	defaultValue, err := toFloat(setting.Default)
	if err != nil {
		return nil, fmt.Errorf("setting '%s': default must be a number", key)
	}
	if setting.Min == nil || setting.Max == nil {
		return nil, fmt.Errorf("setting '%s': a number setting must declare min and max", key)
	}
	step := 1.0
	if setting.Step != nil {
		step = *setting.Step
	}
	return &oapi.GameVersionSettingNumberDTO{
		Min:          *setting.Min,
		Max:          *setting.Max,
		Step:         step,
		DefaultValue: defaultValue,
	}, nil
}

func (setting SettingConfig) selectDTO(key string) (*oapi.GameVersionSettingSelectDTO, error) {
	if len(setting.Options) == 0 {
		return nil, fmt.Errorf("setting '%s': a select setting must declare options", key)
	}

	options := make([]oapi.GameVersionSettingOptionDTO, 0, len(setting.Options))
	for _, rawOption := range setting.Options {
		settingOption, err := toSettingOption(rawOption)
		if err != nil {
			return nil, fmt.Errorf("setting '%s': %w", key, err)
		}
		options = append(options, settingOption)
	}

	defaultValue, err := toStringSlice(setting.Default)
	if err != nil {
		return nil, fmt.Errorf("setting '%s': default must be a string or a list of strings", key)
	}
	if !setting.Multiple && len(defaultValue) > 1 {
		return nil, fmt.Errorf("setting '%s': a select setting without 'multiple = true' takes a single default", key)
	}

	for _, selected := range defaultValue {
		if !containsOptionValue(options, selected) {
			return nil, fmt.Errorf("setting '%s': default '%s' is not one of the declared options", key, selected)
		}
	}

	return &oapi.GameVersionSettingSelectDTO{
		Multiple:     setting.Multiple,
		Options:      options,
		DefaultValue: defaultValue,
	}, nil
}

func (setting SettingConfig) textDTO(key string) (*oapi.GameVersionSettingTextDTO, error) {
	defaultValue, ok := setting.Default.(string)
	if !ok {
		return nil, fmt.Errorf("setting '%s': default must be a string", key)
	}
	return &oapi.GameVersionSettingTextDTO{
		DefaultValue: defaultValue,
		MaxLength:    setting.MaxLength,
	}, nil
}

func toFloat(value any) (float64, error) {
	switch typedValue := value.(type) {
	case int64:
		return float64(typedValue), nil
	case float64:
		return typedValue, nil
	}
	return 0, fmt.Errorf("not a number: %v", value)
}

// toStringSlice accepts both `default = "desert"` and `default = ["desert"]`.
func toStringSlice(value any) ([]string, error) {
	switch typedValue := value.(type) {
	case string:
		return []string{typedValue}, nil
	case []any:
		result := make([]string, 0, len(typedValue))
		for _, element := range typedValue {
			stringElement, ok := element.(string)
			if !ok {
				return nil, fmt.Errorf("not a string: %v", element)
			}
			result = append(result, stringElement)
		}
		return result, nil
	}
	return nil, fmt.Errorf("not a string or list of strings: %v", value)
}

// toSettingOption accepts a bare string, whose label is the value itself, or an
// inline table carrying a separate label: { value = "desert", label = "Desert" }.
func toSettingOption(value any) (oapi.GameVersionSettingOptionDTO, error) {
	switch typedValue := value.(type) {
	case string:
		return oapi.GameVersionSettingOptionDTO{Value: typedValue, Label: typedValue}, nil
	case map[string]any:
		optionValue, ok := typedValue["value"].(string)
		if !ok {
			return oapi.GameVersionSettingOptionDTO{}, fmt.Errorf("option is missing a string 'value'")
		}
		label, ok := typedValue["label"].(string)
		if !ok {
			label = optionValue
		}
		return oapi.GameVersionSettingOptionDTO{Value: optionValue, Label: label}, nil
	}
	return oapi.GameVersionSettingOptionDTO{}, fmt.Errorf("option must be a string or a table with 'value' and 'label'")
}

func containsOptionValue(options []oapi.GameVersionSettingOptionDTO, value string) bool {
	for _, settingOption := range options {
		if settingOption.Value == value {
			return true
		}
	}
	return false
}

// GetSettingDTOs converts every declared setting, reporting the first problem it
// finds so `mugon publish` fails before uploading anything.
func GetSettingDTOs(settings []SettingConfig) ([]oapi.GameVersionSettingsItemDTO, error) {
	result := make([]oapi.GameVersionSettingsItemDTO, 0, len(settings))
	seenKeys := make(map[string]bool, len(settings))
	for _, setting := range settings {
		dto, err := setting.ToDTO()
		if err != nil {
			return nil, err
		}
		if seenKeys[dto.Key] {
			return nil, fmt.Errorf("duplicate setting key '%s'", dto.Key)
		}
		seenKeys[dto.Key] = true
		result = append(result, dto)
	}
	return result, nil
}
