package logging

import (
	"reflect"
	"strings"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
)

func GetValidatorTranslator() ut.Translator {
	english := en.New()
	uni := ut.New(english, english)
	trans, _ := uni.GetTranslator("en")
	return trans
}

func SetupValidatorLogging(validate *validator.Validate, trans ut.Translator) {
	err := en_translations.RegisterDefaultTranslations(validate, trans)
	if err != nil {
		panic(err)
	}
	err = validate.RegisterTranslation("semver", trans, func(ut ut.Translator) error {
		return ut.Add("semver", "{0} must be a valid semantic version (e.g. 0.1.0)", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("semver", fe.Field())
		return t
	})
	if err != nil {
		panic(err)
	}
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("toml"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}
