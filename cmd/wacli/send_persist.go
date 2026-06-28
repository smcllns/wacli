package main

import (
	"fmt"
	"os"
)

func persistStatus(err error) (bool, string) {
	if err == nil {
		return true, ""
	}
	return false, err.Error()
}

func warnPersistFailure(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: message sent but not persisted locally: %v\n", err)
	}
}
