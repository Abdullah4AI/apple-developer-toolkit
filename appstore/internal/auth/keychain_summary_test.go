package auth

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/99designs/keyring"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/config"
)

const testCredentialMetadataDescription = `asc:metadata:{"key_id":"KEY123","issuer_id":"ISS456"}`

type metadataKeyring struct {
	metadata map[string]keyring.Metadata
	items    map[string]keyring.Item
	getCalls int
	setCalls int
}

func (k *metadataKeyring) Get(key string) (keyring.Item, error) {
	k.getCalls++
	if item, ok := k.items[key]; ok {
		return item, nil
	}
	return keyring.Item{}, keyring.ErrKeyNotFound
}

func (k *metadataKeyring) GetMetadata(key string) (keyring.Metadata, error) {
	if metadata, ok := k.metadata[key]; ok {
		return metadata, nil
	}
	return keyring.Metadata{}, keyring.ErrKeyNotFound
}

func (k *metadataKeyring) Set(item keyring.Item) error {
	k.setCalls++
	if k.items == nil {
		k.items = map[string]keyring.Item{}
	}
	if k.metadata == nil {
		k.metadata = map[string]keyring.Metadata{}
	}
	k.items[item.Key] = item
	metadataItem := item
	metadataItem.Data = nil
	k.metadata[item.Key] = keyring.Metadata{Item: &metadataItem}
	return nil
}

func (k *metadataKeyring) Remove(string) error {
	return nil
}

func (k *metadataKeyring) Keys() ([]string, error) {
	keys := make([]string, 0, len(k.metadata))
	for key := range k.metadata {
		keys = append(keys, key)
	}
	return keys, nil
}

func withMetadataKeyring(t *testing.T, kr keyring.Keyring) {
	t.Helper()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "0")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	previousKeyringOpener := keyringOpener
	previousLegacyKeyringOpener := legacyKeyringOpener
	keyringOpener = func() (keyring.Keyring, error) {
		return kr, nil
	}
	legacyKeyringOpener = func() (keyring.Keyring, error) {
		return nil, keyring.ErrNoAvailImpl
	}
	t.Cleanup(func() {
		keyringOpener = previousKeyringOpener
		legacyKeyringOpener = previousLegacyKeyringOpener
	})
}

func TestListCredentialSummaries_UsesKeychainMetadataWithoutGet(t *testing.T) {
	name := "personal"
	kr := &metadataKeyring{
		metadata: map[string]keyring.Metadata{
			keyringKey(name): {
				Item: &keyring.Item{
					Key:         keyringKey(name),
					Label:       "ASC API Key (personal)",
					Description: testCredentialMetadataDescription,
				},
			},
		},
	}
	withMetadataKeyring(t, kr)

	creds, err := ListCredentialSummaries()
	if err != nil {
		t.Fatalf("ListCredentialSummaries() error: %v", err)
	}
	if kr.getCalls != 0 {
		t.Fatalf("expected metadata-only listing to avoid Get(), got %d calls", kr.getCalls)
	}
	if len(creds) != 1 {
		t.Fatalf("expected one credential, got %d", len(creds))
	}
	if creds[0].Name != name {
		t.Fatalf("expected credential name %q, got %q", name, creds[0].Name)
	}
	if creds[0].KeyID != "KEY123" {
		t.Fatalf("expected key ID %q, got %q", "KEY123", creds[0].KeyID)
	}
	if creds[0].IssuerID != "ISS456" {
		t.Fatalf("expected issuer ID %q, got %q", "ISS456", creds[0].IssuerID)
	}
	if !creds[0].IsDefault {
		t.Fatalf("expected single keychain credential to be default, got %#v", creds[0])
	}
}

func TestListCredentialSummaries_LegacyEntriesDoNotReadSecretPayload(t *testing.T) {
	name := "legacy"
	kr := &metadataKeyring{
		metadata: map[string]keyring.Metadata{
			keyringKey(name): {
				Item: &keyring.Item{
					Key:   keyringKey(name),
					Label: "ASC API Key (legacy)",
				},
			},
		},
	}
	withMetadataKeyring(t, kr)

	creds, err := ListCredentialSummaries()
	if err != nil {
		t.Fatalf("ListCredentialSummaries() error: %v", err)
	}
	if kr.getCalls != 0 {
		t.Fatalf("expected legacy summary listing to avoid Get(), got %d calls", kr.getCalls)
	}
	if len(creds) != 1 {
		t.Fatalf("expected one credential, got %d", len(creds))
	}
	if creds[0].KeyID != "" {
		t.Fatalf("expected missing key metadata to stay blank, got %#v", creds[0])
	}
}

func TestInspectProfiles_UsesMetadataOnlyListing(t *testing.T) {
	kr := &metadataKeyring{
		metadata: map[string]keyring.Metadata{
			keyringKey("personal"): {
				Item: &keyring.Item{
					Key:         keyringKey("personal"),
					Label:       "ASC API Key (personal)",
					Description: testCredentialMetadataDescription,
				},
			},
		},
	}
	withMetadataKeyring(t, kr)

	section := inspectProfiles()
	if kr.getCalls != 0 {
		t.Fatalf("expected inspectProfiles() to avoid Get(), got %d calls", kr.getCalls)
	}
	if !sectionHasStatus(section, DoctorOK, "personal - complete (keychain)") {
		t.Fatalf("expected keychain profile summary in doctor output, got %#v", section.Checks)
	}
}

func TestListCredentials_DoesNotRewriteMetadataOnlyMismatchOnFullRead(t *testing.T) {
	name := "default"
	kr := &metadataKeyring{
		metadata: map[string]keyring.Metadata{
			keyringKey(name): {
				Item: &keyring.Item{
					Key:   keyringKey(name),
					Label: "ASC API Key (default)",
				},
			},
		},
		items: map[string]keyring.Item{},
	}
	withMetadataKeyring(t, kr)

	payload := credentialPayload{
		KeyID:         "KEY123",
		IssuerID:      "ISS456",
		PrivateKeyPEM: "-----BEGIN PRIVATE KEY-----\nTEST\n-----END PRIVATE KEY-----\n",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload error: %v", err)
	}
	kr.items[keyringKey(name)] = keyring.Item{
		Key:   keyringKey(name),
		Data:  data,
		Label: "ASC API Key (default)",
	}

	creds, err := ListCredentials()
	if err != nil {
		t.Fatalf("ListCredentials() error: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("expected one credential, got %d", len(creds))
	}

	if kr.setCalls != 0 {
		t.Fatalf("expected metadata-only mismatch to avoid keychain writes, got %d Set() calls", kr.setCalls)
	}
}

func TestListCredentialSummaries_IgnoresStaleStoredMetadata(t *testing.T) {
	modifiedAt := time.Date(2026, 3, 15, 4, 45, 0, 0, time.UTC)
	kr := &metadataKeyring{
		metadata: map[string]keyring.Metadata{
			keyringKey("legacy"): {
				Item: &keyring.Item{
					Key:   keyringKey("legacy"),
					Label: "ASC API Key (legacy)",
				},
				ModificationTime: modifiedAt,
			},
		},
	}
	withMetadataKeyring(t, kr)

	path, err := config.Path()
	if err != nil {
		t.Fatalf("config.Path() error: %v", err)
	}
	if err := config.SaveAt(path, &config.Config{
		KeychainMetadata: []config.KeychainMetadata{{
			Name:       "legacy",
			KeyID:      "OLDKEY",
			IssuerID:   "OLDISS",
			ModifiedAt: metadataModifiedAtString(modifiedAt.Add(-time.Minute)),
		}},
	}); err != nil {
		t.Fatalf("config.SaveAt() error: %v", err)
	}

	creds, err := ListCredentialSummaries()
	if err != nil {
		t.Fatalf("ListCredentialSummaries() error: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("expected one credential, got %d", len(creds))
	}
	if creds[0].KeyID != "" || creds[0].IssuerID != "" {
		t.Fatalf("expected stale stored metadata to be ignored, got %#v", creds[0])
	}
}
