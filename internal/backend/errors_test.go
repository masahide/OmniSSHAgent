package backend

import (
	"errors"
	"testing"
)

func TestErrorClassification(t *testing.T) {
	err := &Error{Kind: ErrorTimeout, Operation: "connect", Err: errors.New("deadline")}
	if !IsKind(err, ErrorTimeout) || IsKind(err, ErrorUnavailable) {
		t.Fatal("classification failed")
	}
	if !errors.Is(err, err.Err) {
		t.Fatal("cause was not preserved")
	}
}
