//go:build darwin

package web

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWebSignInKeysDoesNotInheritReadAccess(t *testing.T) {
	directory := t.TempDir()
	output, err := exec.Command("/bin/chmod", "+a", "everyone allow read,file_inherit", directory).CombinedOutput()
	if err != nil {
		t.Fatalf("apply inheritable ACL: %v (%s)", err, output)
	}
	testWebSignInKeysCreateSavesPrivateFileAndReceipt(t, directory)
	output, err = exec.Command("/bin/ls", "-le", filepath.Join(directory, "AuthKey_KEY123.p8")).CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && strings.HasSuffix(fields[0], ":") {
			t.Fatal("private key retained an inherited ACL")
		}
	}
}
