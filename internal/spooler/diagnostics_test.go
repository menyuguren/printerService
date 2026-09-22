package spooler

import (
	"errors"
	"strings"
	"testing"
)

func TestStageErrorIncludesJobID(t *testing.T) {
	err := stageError("EndDocPrinter", 42, errors.New("access denied"))

	if !strings.Contains(err.Error(), "EndDocPrinter job 42") {
		t.Fatalf("error = %q, want stage and job ID", err)
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Fatalf("error = %q, want underlying error", err)
	}
}
