package main

import (
	"errors"
	"fmt"
)

type ValidationError struct{ Field string }

func (e *ValidationError) Error() string { return "invalid " + e.Field }

func main() {
	base := &ValidationError{Field: "email"}

	e1 := fmt.Errorf("signup: %w", base)
	e2 := fmt.Errorf("signup: %v", base)

	var ve *ValidationError
	fmt.Println(errors.As(e1, &ve))
	fmt.Println(errors.As(e2, &ve))
}
