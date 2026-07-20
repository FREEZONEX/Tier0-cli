package errs

import (
	"errors"
	"testing"
)

func TestInvalidArgumentEnvelopeIncludesContractFields(t *testing.T) {
	cause := errors.New("bad value")
	err := InvalidArgument("--qos", "--qos must be 0, 1, or 2").WithCause(cause)

	if !errors.Is(err, cause) {
		t.Fatal("typed error must preserve its cause")
	}
	envelope := BuildEnvelope(err)
	if envelope.OK {
		t.Fatal("error envelope must set ok=false")
	}
	if envelope.Error.Type != CategoryValidation {
		t.Fatalf("type = %q, want %q", envelope.Error.Type, CategoryValidation)
	}
	if envelope.Error.Subtype != SubtypeInvalidArgument {
		t.Fatalf("subtype = %q, want %q", envelope.Error.Subtype, SubtypeInvalidArgument)
	}
	if envelope.Error.Param != "--qos" {
		t.Fatalf("param = %q, want --qos", envelope.Error.Param)
	}
}

func TestFileIOPreservesCause(t *testing.T) {
	cause := errors.New("file not found")
	err := FileIO("--file", "failed to read file", cause)

	if err.Cat != CategoryInternal || err.Subtype != SubtypeFileIO {
		t.Fatalf("unexpected classification: %#v", err)
	}
	if !errors.Is(err, cause) {
		t.Fatal("file I/O error must preserve its cause")
	}
}
