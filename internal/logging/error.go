package logging

import (
	"fmt"
	"os"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
)

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
