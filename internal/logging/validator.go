package logging

import (
	"fmt"
	"os"
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

func PrintUserFacingErrorMessage(err error, trans ut.Translator) {
	if errs, ok := err.(validator.ValidationErrors); ok {
		fmt.Println("Error when reading config file:")
		for _, e := range errs {
			fmt.Println(e.Translate(trans))
		}
	} else if err.Error() != "user aborted" {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}
}

func SetupValidatorLogging(validate *validator.Validate, trans ut.Translator) {
	en_translations.RegisterDefaultTranslations(validate, trans)
	validate.RegisterTranslation("semver", trans, func(ut ut.Translator) error {
		return ut.Add("semver", "{0} must be a valid semantic version (e.g. 0.1.0)", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("semver", fe.Field())
		return t
	})
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("toml"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}
