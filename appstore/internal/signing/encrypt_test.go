package signing

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plaintext := []byte("Hello, signing sync!")
	password := "test-password-123"

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if bytes.Equal(encrypted, plaintext) {
		t.Fatal("encrypted data should differ from plaintext")
	}

	decrypted, err := Decrypt(encrypted, password)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted data doesn't match: got %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptWrongPassword(t *testing.T) {
	plaintext := []byte("secret data")
	encrypted, err := Encrypt(plaintext, "correct-password")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	_, err = Decrypt(encrypted, "wrong-password")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestDecryptTooShort(t *testing.T) {
	_, err := Decrypt([]byte("short"), "password")
	if err == nil {
		t.Fatal("expected error for short data")
	}
}

func TestEncryptProducesDifferentOutput(t *testing.T) {
	plaintext := []byte("same input")
	password := "same-password"

	enc1, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("encrypt 1: %v", err)
	}

	enc2, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("encrypt 2: %v", err)
	}

	if bytes.Equal(enc1, enc2) {
		t.Fatal("two encryptions of same data should produce different output (random salt/nonce)")
	}
}

func TestEncryptEmptyData(t *testing.T) {
	encrypted, err := Encrypt([]byte{}, "password")
	if err != nil {
		t.Fatalf("encrypt empty: %v", err)
	}

	decrypted, err := Decrypt(encrypted, "password")
	if err != nil {
		t.Fatalf("decrypt empty: %v", err)
	}

	if len(decrypted) != 0 {
		t.Fatalf("expected empty, got %d bytes", len(decrypted))
	}
}
