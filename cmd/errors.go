package cmd

import (
	"errors"
	"fmt"
	"os"
)

type ExitError struct {
	Code   int
	Msg    string
	Silent bool
}

func (e *ExitError) Error() string {
	return e.Msg
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}

	var exitErr *ExitError
	if errors.As(err, &exitErr) && exitErr.Code != 0 {
		return exitErr.Code
	}

	return 1
}

func PrintError(err error) {
	if err == nil {
		return
	}

	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		if exitErr.Silent || exitErr.Msg == "" {
			return
		}
		fmt.Fprintln(os.Stderr, exitErr.Msg)
		return
	}

	fmt.Fprintln(os.Stderr, err.Error())
}
