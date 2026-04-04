package shared

import "testing"

func TestMultiStringFlagSetAppendsTrimmedValues(t *testing.T) {
	var flag MultiStringFlag

	if err := flag.Set("  -quiet  "); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := flag.Set("-allowProvisioningUpdates"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, want := flag.String(), "-quiet,-allowProvisioningUpdates"; got != want {
		t.Fatalf("unexpected string form: got %q want %q", got, want)
	}
}

func TestMultiStringFlagSetRejectsEmptyValues(t *testing.T) {
	var flag MultiStringFlag

	if err := flag.Set("   "); err == nil {
		t.Fatal("expected empty value error")
	}
}
