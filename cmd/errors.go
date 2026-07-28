package cmd

import (
	"errors"
	"fmt"
)

// invalidArgsError marks a command error as a caller-input failure so the
// process can return ExitInvalidArgs rather than a general error.
type invalidArgsError struct {
	err error
}

func (e *invalidArgsError) Error() string { return e.err.Error() }

func (e *invalidArgsError) Unwrap() error { return e.err }

func invalidArgs(err error) error {
	return &invalidArgsError{err: err}
}

func invalidArgsf(format string, args ...interface{}) error {
	return invalidArgs(fmt.Errorf(format, args...))
}

func isInvalidArgsError(err error) bool {
	var target *invalidArgsError
	return errors.As(err, &target)
}
