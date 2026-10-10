package asc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestIOSArtifactReceiptStatesVerificationBoundary(t *testing.T) {
	result := &IOSArtifactResult{Operation: "package", Backend: "rcodesign", Platform: "device", SigningType: "adHoc", AppleAcceptance: "notVerified", Success: true}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["signatureVerified"] != false || decoded["appleAcceptance"] != "notVerified" {
		t.Fatalf("misleading receipt: %s", data)
	}
	ensureOutputRegistryPopulated()
	if !isRegistryTypeRegistered(typeForPtr[IOSArtifactResult]()) {
		t.Fatal("missing renderer")
	}
	for _, render := range []func(any) error{PrintTable, PrintMarkdown} {
		text := captureStdout(t, func() error { return render(result) })
		if !strings.Contains(text, "signature_verified") || !strings.Contains(text, "notVerified") {
			t.Fatalf("missing boundary: %s", text)
		}
	}
}
