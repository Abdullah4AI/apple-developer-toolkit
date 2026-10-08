package web

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

func TestIndividualAPIKeyMatchesValidatedPublicKey(t *testing.T) {
	publicPEM := testIndividualPublicKeyPEM(t)
	resource := individualAPIKeyResource{Type: individualAPIKeyResourceType, ID: "TEST-KEY"}
	resource.Attributes.PublicKey = &publicPEM
	key, err := decodeIndividualAPIKeyResource(resource)
	if err != nil {
		t.Fatal(err)
	}
	for name, input := range map[string]string{
		"original":               publicPEM,
		"CRLF":                   strings.ReplaceAll(publicPEM, "\n", "\r\n"),
		"surrounding whitespace": "\n" + publicPEM + "\n\t",
	} {
		t.Run(name, func(t *testing.T) {
			if !key.MatchesPublicKey(input) {
				t.Fatal("equivalent public key did not match")
			}
		})
	}
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	for name, input := range map[string]string{
		"different public key": testIndividualPublicKeyPEM(t),
		"invalid PEM":          "invalid",
		"invalid DER":          string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte("invalid")})),
		"private key":          string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})),
		"trailing material":    publicPEM + "trailing",
	} {
		t.Run(name, func(t *testing.T) {
			if key.MatchesPublicKey(input) {
				t.Fatal("invalid or different key matched")
			}
		})
	}
	if (IndividualAPIKey{}).MatchesPublicKey(publicPEM) {
		t.Fatal("resource without a registered public key matched")
	}
}
