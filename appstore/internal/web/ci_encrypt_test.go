package web

import (
	"crypto/ecdh"
	"encoding/base64"
	"testing"
)

func TestECIESEncrypt_OutputStructure(t *testing.T) {
	// Use the real server key from the CI API
	serverKeyB64 := "0xm9f0gX7lzArxrChNrDVUR3MKxueb1DdheWBeLndCVOqoiEsT2jxqZW6cHsIuDGDykvYWgQ1qaPBSxCNFXEUg=="

	plaintext := "go-ecies-test-value"
	ct, err := ECIESEncrypt(serverKeyB64, plaintext)
	if err != nil {
		t.Fatalf("ECIESEncrypt failed: %v", err)
	}

	// Decode and verify structure
	raw, err := base64.StdEncoding.DecodeString(ct)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	expectedLen := 32 + 64 + 12 + len(plaintext) + 16 // salt + pub + iv + plaintext + gcm tag
	if len(raw) != expectedLen {
		t.Errorf("expected %d bytes, got %d", expectedLen, len(raw))
	}

	t.Logf("Ciphertext length: %d base64 chars, %d raw bytes", len(ct), len(raw))
	t.Logf("Ciphertext: %s", ct)

	// Verify parts are extractable
	salt := raw[:32]
	ephPub := raw[32:96]
	iv := raw[96:108]
	encData := raw[108:]

	t.Logf("Salt (%d bytes): %x", len(salt), salt)
	t.Logf("Ephemeral pub (%d bytes): %x...", len(ephPub), ephPub[:8])
	t.Logf("IV (%d bytes): %x", len(iv), iv)
	t.Logf("Encrypted data (%d bytes): %x...", len(encData), encData[:8])

	// Verify ephemeral public key is a valid P-256 point
	uncompressed := make([]byte, 65)
	uncompressed[0] = 0x04
	copy(uncompressed[1:], ephPub)
	_, err = ecdh.P256().NewPublicKey(uncompressed)
	if err != nil {
		t.Errorf("ephemeral public key is not a valid P-256 point: %v", err)
	}
}

func TestECIESEncrypt_DifferentEachTime(t *testing.T) {
	serverKeyB64 := "0xm9f0gX7lzArxrChNrDVUR3MKxueb1DdheWBeLndCVOqoiEsT2jxqZW6cHsIuDGDykvYWgQ1qaPBSxCNFXEUg=="
	plaintext := "same-input"

	ct1, err := ECIESEncrypt(serverKeyB64, plaintext)
	if err != nil {
		t.Fatalf("encrypt 1 failed: %v", err)
	}
	ct2, err := ECIESEncrypt(serverKeyB64, plaintext)
	if err != nil {
		t.Fatalf("encrypt 2 failed: %v", err)
	}

	if ct1 == ct2 {
		t.Error("two encryptions of the same plaintext should produce different ciphertexts")
	}
}
