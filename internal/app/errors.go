package app

import (
	"errors"
	"fmt"
)

type InputError struct {
	message string
}

func (e *InputError) Error() string {
	return e.message
}

func NewInputErrorf(format string, args ...any) error {
	return &InputError{message: fmt.Sprintf(format, args...)}
}

func IsInputError(err error) bool {
	var inputErr *InputError
	return errors.As(err, &inputErr)
}
