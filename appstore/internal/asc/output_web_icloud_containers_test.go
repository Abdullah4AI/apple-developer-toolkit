package asc

import (
	"strings"
	"testing"
)

func TestPrintTableWebICloudContainerCreateUsesRegistry(t *testing.T) {
	result := &WebICloudContainerCreateResult{
		Operation:   "create",
		ContainerID: "cloud-1",
		Identifier:  "iCloud.com.example.app",
		Name:        "Example Container",
		Prefix:      "TEAM123456",
		Changed:     true,
		Verified:    true,
		Permanent:   true,
		Status:      "created",
	}
	for _, render := range []func(any) error{PrintTable, PrintMarkdown} {
		output := captureStdout(t, func() error { return render(result) })
		for _, want := range []string{"Container ID", "Permanent", "create", "cloud-1", "iCloud.com.example.app", "Example Container", "TEAM123456", "created"} {
			if !strings.Contains(output, want) {
				t.Fatalf("output missing %q: %q", want, output)
			}
		}
	}
}
