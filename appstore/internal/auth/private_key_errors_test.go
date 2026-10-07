package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPrivateKeyErrorsCarryStructuredKinds(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.p8")
		err := ValidateKeyFile(path)
		assertPrivateKeyErrorKind(t, err, PrivateKeyNotFound)
		if got, want := err.Error(), fmt.Sprintf("private key file not found: %q", path); got != want {
			t.Fatalf("error = %q, want %q", got, want)
		}
	})

	t.Run("invalid pem", func(t *testing.T) {
		_, err := LoadPrivateKeyFromPEM([]byte("not pem"))
		assertPrivateKeyErrorKind(t, err, PrivateKeyInvalidFormat)
		if got, want := err.Error(), "invalid PEM data"; got != want {
			t.Fatalf("error = %q, want %q", got, want)
		}
	})

	t.Run("unsupported algorithm", func(t *testing.T) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("generate RSA key: %v", err)
		}
		der, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			t.Fatalf("marshal RSA key: %v", err)
		}
		data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
		_, err = LoadPrivateKeyFromPEM(data)
		assertPrivateKeyErrorKind(t, err, PrivateKeyUnsupportedAlgorithm)
		if got, want := err.Error(), "private key is not ECDSA"; got != want {
			t.Fatalf("error = %q, want %q", got, want)
		}
	})

	t.Run("unsupported algorithm pkcs1", func(t *testing.T) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("generate RSA key: %v", err)
		}
		data := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
		_, err = LoadPrivateKeyFromPEM(data)
		assertPrivateKeyErrorKind(t, err, PrivateKeyUnsupportedAlgorithm)
		if got, want := err.Error(), "private key is not ECDSA"; got != want {
			t.Fatalf("error = %q, want %q", got, want)
		}

		path := filepath.Join(t.TempDir(), "rsa.p8")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("write RSA key: %v", err)
		}
		err = ValidateKeyFile(path)
		assertPrivateKeyErrorKind(t, err, PrivateKeyUnsupportedAlgorithm)
	})

	for _, pkcs8 := range []bool{true, false} {
		name := "unsupported curve sec1"
		if pkcs8 {
			name = "unsupported curve pkcs8"
		}
		t.Run(name, func(t *testing.T) {
			key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
			if err != nil {
				t.Fatalf("generate P-384 key: %v", err)
			}
			var der []byte
			if pkcs8 {
				der, err = x509.MarshalPKCS8PrivateKey(key)
			} else {
				der, err = x509.MarshalECPrivateKey(key)
			}
			if err != nil {
				t.Fatalf("marshal P-384 key: %v", err)
			}
			data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
			_, err = LoadPrivateKeyFromPEM(data)
			assertPrivateKeyErrorKind(t, err, PrivateKeyUnsupportedAlgorithm)
			if got, want := err.Error(), "private key must use the P-256 curve"; got != want {
				t.Fatalf("error = %q, want %q", got, want)
			}
		})
	}

	t.Run("insecure permissions", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows does not expose POSIX key permissions")
		}
		path := filepath.Join(t.TempDir(), "key.p8")
		if err := os.WriteFile(path, []byte("not pem"), 0o644); err != nil {
			t.Fatalf("write key: %v", err)
		}
		if err := os.Chmod(path, 0o644); err != nil {
			t.Fatalf("set key permissions: %v", err)
		}
		err := ValidateKeyFile(path)
		assertPrivateKeyErrorKind(t, err, PrivateKeyPermissionsInsecure)
		if !strings.Contains(err.Error(), "private key file is too permissive") {
			t.Fatalf("error message changed: %v", err)
		}
	})
}

func assertPrivateKeyErrorKind(t *testing.T, err error, want PrivateKeyErrorKind) {
	t.Helper()
	if err == nil {
		t.Fatal("error = nil")
	}
	got, ok := PrivateKeyErrorKindOf(err)
	if !ok || got != want {
		t.Fatalf("PrivateKeyErrorKindOf(%v) = %q, %t; want %q, true", err, got, ok, want)
	}
	var keyErr *PrivateKeyError
	if !errors.As(err, &keyErr) {
		t.Fatalf("errors.As(%v, *PrivateKeyError) = false", err)
	}
}
