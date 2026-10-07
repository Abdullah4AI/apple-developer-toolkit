package storekit

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// DecodeJWSPayloadUnverified returns the JSON payload of an Apple-signed
// compact JWS. It does NOT verify the signature or certificate chain, so the
// result must not be trusted as proof of a purchase.
func DecodeJWSPayloadUnverified(token string) (json.RawMessage, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("JWS must have three dot-separated parts")
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return nil, fmt.Errorf("decode JWS payload: %w", err)
	}
	if !json.Valid(payload) {
		return nil, fmt.Errorf("JWS payload is not JSON")
	}
	return payload, nil
}

// AddDecodedJWSPayloads returns raw with a "<field>Decoded" sibling next to
// every signed* string or string-array field, holding the unverified payload.
// Signed fields inside decoded payloads are decoded too. Existing fields are
// never changed.
func AddDecodedJWSPayloads(raw json.RawMessage) (json.RawMessage, error) {
	value, err := decodeJSONNumbers(raw)
	if err != nil {
		return nil, fmt.Errorf("decode StoreKit response: %w", err)
	}
	decorated, err := addDecodedPayloads(value)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(decorated)
	if err != nil {
		return nil, fmt.Errorf("encode decoded StoreKit response: %w", err)
	}
	return encoded, nil
}

func addDecodedPayloads(value any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		decodedFields := map[string]any{}
		for key, field := range typed {
			walked, err := addDecodedPayloads(field)
			if err != nil {
				return nil, err
			}
			typed[key] = walked
			if !strings.HasPrefix(key, "signed") {
				continue
			}
			decoded, ok, err := decodeSignedField(walked)
			if err != nil {
				return nil, fmt.Errorf("decode %s: %w", key, err)
			}
			if ok {
				decodedFields[key+"Decoded"] = decoded
			}
		}
		for key, decoded := range decodedFields {
			if _, exists := typed[key]; !exists {
				typed[key] = decoded
			}
		}
		return typed, nil
	case []any:
		for i, item := range typed {
			walked, err := addDecodedPayloads(item)
			if err != nil {
				return nil, err
			}
			typed[i] = walked
		}
		return typed, nil
	default:
		return value, nil
	}
}

// decodeSignedField decodes a JWS string or an array of JWS strings. Other
// shapes, such as the numeric signedDate claim, are left alone.
func decodeSignedField(field any) (any, bool, error) {
	switch typed := field.(type) {
	case string:
		decoded, err := decodeJWSValue(typed)
		return decoded, err == nil, err
	case []any:
		decoded := make([]any, 0, len(typed))
		for _, item := range typed {
			token, ok := item.(string)
			if !ok {
				return nil, false, nil
			}
			value, err := decodeJWSValue(token)
			if err != nil {
				return nil, false, err
			}
			decoded = append(decoded, value)
		}
		return decoded, true, nil
	default:
		return nil, false, nil
	}
}

func decodeJWSValue(token string) (any, error) {
	payload, err := DecodeJWSPayloadUnverified(token)
	if err != nil {
		return nil, err
	}
	value, err := decodeJSONNumbers(payload)
	if err != nil {
		return nil, fmt.Errorf("decode JWS payload: %w", err)
	}
	return addDecodedPayloads(value)
}

// decodeJSONNumbers keeps large integers such as millisecond dates exact.
func decodeJSONNumbers(raw []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	err := decoder.Decode(&value)
	return value, err
}
