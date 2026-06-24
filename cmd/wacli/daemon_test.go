package main

import (
	"strings"
	"testing"
)

func TestDaemonCommandRequiresSocket(t *testing.T) {
	err := execute([]string{"daemon", "--socket", ""})
	if err == nil || !strings.Contains(err.Error(), "--socket is required") {
		t.Fatalf("err = %v, want socket requirement", err)
	}
}
