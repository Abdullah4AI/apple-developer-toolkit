package asc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
)

func benchmarkClientWithKey(b *testing.B) *Client {
	b.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		b.Fatalf("GenerateKey() error: %v", err)
	}

	return &Client{
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: key,
	}
}

func BenchmarkClientNewRequest(b *testing.B) {
	client := benchmarkClientWithKey(b)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, err := client.newRequest(ctx, "GET", "/v1/apps", nil)
		if err != nil {
			b.Fatalf("newRequest() error: %v", err)
		}
		if req.Header.Get("Authorization") == "" {
			b.Fatal("expected Authorization header to be set")
		}
	}
}
