package asc

import (
	"reflect"
	"testing"
)

func TestArtifactInfoRowsAddVerificationDetailOnlyWhenPresent(t *testing.T) {
	headers, rows := artifactIPAInfoRows(&ArtifactIPAInfo{SignatureVerification: "not-verified"})
	if len(headers) != 8 || len(rows[0]) != 8 || headers[7] != "Architectures" {
		t.Fatalf("default table changed: %v %v", headers, rows)
	}
	headers, rows = artifactIPAInfoRows(&ArtifactIPAInfo{SignatureVerification: "expired", SignatureVerificationDetail: "certificate expired", Architectures: []ArtifactArchitecture{{Arch: "arm64"}}})
	if !reflect.DeepEqual(headers[6:], []string{"Signature", "Architectures", "Signature Detail"}) || !reflect.DeepEqual(rows[0][6:], []string{"expired", "arm64", "certificate expired"}) {
		t.Fatalf("headers=%v rows=%v", headers, rows)
	}
	headers, rows = artifactPKGInfoRows(&ArtifactPKGInfo{SignatureVerification: "invalid", SignatureVerificationDetail: "package is unsigned"})
	if headers[len(headers)-1] != "Signature Detail" || !reflect.DeepEqual(rows[0][len(rows[0])-2:], []string{"invalid", "package is unsigned"}) {
		t.Fatalf("headers=%v rows=%v", headers, rows)
	}
}
