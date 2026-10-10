package web

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const individualAPIKeyResourceType = "apiKeys"

// WebUser is the minimal identity returned by the web-session users endpoint.
// It intentionally contains no session or credential material.
type WebUser struct {
	ID       string
	Username string
}

// IndividualAPIKey is the non-secret state needed by the individual-key
// creation flow. PublicKeyPresent reports whether Apple's resource has a
// registered public key without retaining or exposing the key bytes.
type IndividualAPIKey struct {
	KeyID            string
	Active           bool
	PublicKeyPresent bool

	publicKeyFingerprint    [sha256.Size]byte
	publicKeyFingerprintSet bool
}

type individualAPIKeyResource struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Attributes struct {
		IsActive  bool    `json:"isActive"`
		PublicKey *string `json:"publicKey"`
	} `json:"attributes"`
}

type individualAPIKeyListPayload struct {
	Data  []individualAPIKeyResource `json:"data"`
	Links map[string]any             `json:"links"`
}

// MatchesPublicKey reports whether the resource's registered public key is
// the supplied key. The comparison is over the validated DER bytes, so PEM
// line-ending and wrapping differences do not change the result.
func (key IndividualAPIKey) MatchesPublicKey(publicKeyPEM string) bool {
	if !key.PublicKeyPresent || !key.publicKeyFingerprintSet {
		return false
	}
	fingerprint, err := individualAPIKeyPublicKeyFingerprint(publicKeyPEM)
	return err == nil && fingerprint == key.publicKeyFingerprint
}

// GetWebUser returns the web-session user resource for a supplied user id.
// The caller is responsible for comparing Username with the authenticated
// session identity before performing mutations.
func (c *Client) GetWebUser(ctx context.Context, userID string) (*WebUser, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user id is required")
	}

	body, err := c.doIrisV1Request(ctx, http.MethodGet, "/users/"+url.PathEscape(userID), nil)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				Username string `json:"username"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse web user response: %w", err)
	}
	if strings.TrimSpace(payload.Data.Type) != "users" {
		return nil, fmt.Errorf("web user response contained resource type %q, want users", strings.TrimSpace(payload.Data.Type))
	}
	responseID := strings.TrimSpace(payload.Data.ID)
	if responseID == "" {
		return nil, fmt.Errorf("web user response did not include a user id")
	}
	if responseID != userID {
		return nil, fmt.Errorf("web user response id %q did not match requested user id %q", responseID, userID)
	}
	username := strings.TrimSpace(payload.Data.Attributes.Username)
	if username == "" {
		return nil, fmt.Errorf("web user response did not include a username")
	}
	return &WebUser{ID: responseID, Username: username}, nil
}

// ListIndividualAPIKeysForUser returns the individual keys Apple associates
// with one user actor. The actor filter is part of the request contract and is
// deliberately separate from the broad visible-key list used by ListAPIKeys.
func (c *Client) ListIndividualAPIKeysForUser(ctx context.Context, userID string) ([]IndividualAPIKey, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user id is required")
	}
	query := url.Values{}
	query.Set("include", "createdByActor,revokedByActor")
	query.Set("filter[createdByActor]", "USER:"+userID)
	nextPath := "/apiKeys?" + query.Encode()
	visited := map[string]struct{}{}
	keys := make([]IndividualAPIKey, 0)
	pagesFetched := 0
	for nextPath != "" {
		if _, seen := visited[nextPath]; seen {
			return nil, fmt.Errorf("individual API keys pagination loop detected")
		}
		visited[nextPath] = struct{}{}

		body, err := c.doIrisV2Request(ctx, http.MethodGet, nextPath, nil)
		if err != nil {
			return nil, wrapAPIKeyListPageError(err, pagesFetched)
		}
		var payload individualAPIKeyListPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("failed to parse individual API keys response: %w", err)
		}
		for index, resource := range payload.Data {
			key, err := decodeIndividualAPIKeyResource(resource)
			if err != nil {
				return nil, fmt.Errorf("individual API key response item %d on page %d: %w", index, pagesFetched+1, err)
			}
			keys = append(keys, *key)
		}
		pagesFetched++
		nextPath, err = nextLookupPagePath(payload.Links, irisV2BaseURL, "individual API keys")
		if err != nil {
			return nil, err
		}
	}
	return keys, nil
}

// CreateIndividualAPIKey creates the empty individual-key resource used by
// Apple's web flow. Apple does not return the created resource in this
// response; callers resolve it with the actor-filtered list before registering
// a public key.
func (c *Client) CreateIndividualAPIKey(ctx context.Context) error {
	request := struct {
		Data struct {
			Type string `json:"type"`
		} `json:"data"`
	}{}
	request.Data.Type = individualAPIKeyResourceType
	_, err := c.doIrisV2Request(ctx, http.MethodPost, "/apiKeys", request)
	return err
}

// RegisterIndividualAPIKey registers a generated public key on an empty
// individual-key resource. The operation is intentionally one-shot; callers
// decide how to handle an uncertain response and must retain local material.
func (c *Client) RegisterIndividualAPIKey(ctx context.Context, keyID, publicKeyPEM string) error {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return fmt.Errorf("api key id is required")
	}
	if strings.TrimSpace(publicKeyPEM) == "" {
		return fmt.Errorf("public key is required")
	}
	request := struct {
		Data struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				PublicKey string `json:"publicKey"`
			} `json:"attributes"`
		} `json:"data"`
	}{}
	request.Data.Type = individualAPIKeyResourceType
	request.Data.ID = keyID
	request.Data.Attributes.PublicKey = publicKeyPEM
	_, err := c.doIrisV2Request(ctx, http.MethodPatch, "/apiKeys/"+url.PathEscape(keyID), request)
	return err
}

func decodeIndividualAPIKeyResource(resource individualAPIKeyResource) (*IndividualAPIKey, error) {
	if strings.TrimSpace(resource.Type) != individualAPIKeyResourceType {
		return nil, fmt.Errorf("resource type %q is not %q", strings.TrimSpace(resource.Type), individualAPIKeyResourceType)
	}
	keyID := strings.TrimSpace(resource.ID)
	if keyID == "" {
		return nil, fmt.Errorf("resource did not include an api key id")
	}
	publicKeyPresent := false
	var publicKeyFingerprint [sha256.Size]byte
	if resource.Attributes.PublicKey != nil && strings.TrimSpace(*resource.Attributes.PublicKey) != "" {
		publicKeyPresent = true
		var err error
		publicKeyFingerprint, err = individualAPIKeyPublicKeyFingerprint(*resource.Attributes.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("resource public key: %w", err)
		}
	}
	return &IndividualAPIKey{
		KeyID:                   keyID,
		Active:                  resource.Attributes.IsActive,
		PublicKeyPresent:        publicKeyPresent,
		publicKeyFingerprint:    publicKeyFingerprint,
		publicKeyFingerprintSet: publicKeyPresent,
	}, nil
}

func individualAPIKeyPublicKeyFingerprint(publicKeyPEM string) ([sha256.Size]byte, error) {
	block, rest := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return [sha256.Size]byte{}, fmt.Errorf("public key is not valid PEM")
	}
	if block.Type != "PUBLIC KEY" {
		return [sha256.Size]byte{}, fmt.Errorf("public key PEM block type %q is not PUBLIC KEY", block.Type)
	}
	if strings.TrimSpace(string(rest)) != "" {
		return [sha256.Size]byte{}, fmt.Errorf("public key PEM contains trailing data")
	}
	if _, err := x509.ParsePKIXPublicKey(block.Bytes); err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("parse public key DER: %w", err)
	}
	return sha256.Sum256(block.Bytes), nil
}
