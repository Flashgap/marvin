package validate

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

// Validator represents the ability to validate itself.
// It's useful for overriding the default behavior of the Validate function.
type Validator interface {
	Validate() error
}

// Struct validates the object using a new go-playground/validator.
// This behavior can be overridden by implementing Validator interface.
func Struct(value any) error {
	if f, ok := value.(Validator); ok {
		return f.Validate()
	}

	v := validator.New()
	if err := v.Struct(value); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	return nil
}
