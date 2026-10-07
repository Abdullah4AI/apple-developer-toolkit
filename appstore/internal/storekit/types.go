// Package storekit implements Apple's In-App Purchase and StoreKit server APIs:
// the App Store Server API and the Retention Messaging API.
//
// StoreKit API credentials are intentionally separate from App Store Connect
// API credentials. The client signs a fresh JWT for every request.
package storekit

import (
	"fmt"
	"strings"
)

const (
	ProductionBaseURL = "https://api.storekit.apple.com"
	SandboxBaseURL    = "https://api.storekit-sandbox.apple.com"
)

// Environment selects the StoreKit server environment.
type Environment string

const (
	Production Environment = "production"
	Sandbox    Environment = "sandbox"
)

// ParseEnvironment validates a StoreKit environment.
func ParseEnvironment(value string) (Environment, error) {
	switch Environment(strings.ToLower(strings.TrimSpace(value))) {
	case Production:
		return Production, nil
	case Sandbox:
		return Sandbox, nil
	default:
		return "", fmt.Errorf("environment must be one of: production, sandbox")
	}
}

func (e Environment) baseURL() string {
	if e == Sandbox {
		return SandboxBaseURL
	}
	return ProductionBaseURL
}

// Credentials holds an In-App Purchase API key and its target app bundle ID.
type Credentials struct {
	KeyID          string
	IssuerID       string
	PrivateKeyPath string
	PrivateKeyPEM  string
	BundleID       string
	Profile        string
}

// ImageSize is the placement for a Retention Messaging image.
type ImageSize string

const (
	ImageSizeFull        ImageSize = "FULL_SIZE"
	ImageSizeBulletPoint ImageSize = "BULLET_POINT"
)

// ImageState is Apple's review state for an uploaded image.
type ImageState string

const (
	StatePending  = "PENDING"
	StateApproved = "APPROVED"
	StateRejected = "REJECTED"
)

type ImageIdentifier struct {
	ImageIdentifier string    `json:"imageIdentifier"`
	ImageSize       ImageSize `json:"imageSize"`
	ImageState      string    `json:"imageState"`
}

type ImageListResponse struct {
	ImageIdentifiers []ImageIdentifier `json:"imageIdentifiers"`
}

// HeaderPosition controls where the header appears in a retention message.
type HeaderPosition string

const (
	HeaderAboveBody  HeaderPosition = "ABOVE_BODY"
	HeaderAboveImage HeaderPosition = "ABOVE_IMAGE"
)

type MessageImage struct {
	ImageIdentifier string `json:"imageIdentifier"`
	AltText         string `json:"altText"`
}

type MessageBulletPoint struct {
	ImageIdentifier string `json:"imageIdentifier"`
	AltText         string `json:"altText"`
	Text            string `json:"text"`
}

type Message struct {
	Header         string               `json:"header"`
	Body           string               `json:"body"`
	Image          *MessageImage        `json:"image,omitempty"`
	BulletPoints   []MessageBulletPoint `json:"bulletPoints,omitempty"`
	HeaderPosition HeaderPosition       `json:"headerPosition,omitempty"`
}

type MessageIdentifier struct {
	MessageIdentifier string `json:"messageIdentifier"`
	MessageState      string `json:"messageState"`
}

type MessageListResponse struct {
	MessageIdentifiers []MessageIdentifier `json:"messageIdentifiers"`
}

type DefaultMessageResponse struct {
	MessageIdentifier string `json:"messageIdentifier"`
}

type RealtimeURLResponse struct {
	RealtimeURL string `json:"realtimeURL"`
}

type PerformanceTestConfig struct {
	MaxConcurrentRequests int64 `json:"maxConcurrentRequests"`
	ResponseTimeThreshold int64 `json:"responseTimeThreshold"`
	SuccessRateThreshold  int32 `json:"successRateThreshold"`
	TotalDuration         int64 `json:"totalDuration"`
	TotalRequests         int   `json:"totalRequests"`
}

type PerformanceTestStartResponse struct {
	Config    PerformanceTestConfig `json:"config"`
	RequestID string                `json:"requestId"`
}

type PerformanceResponseTimes struct {
	Average int64 `json:"average,omitempty"`
	P50     int64 `json:"p50,omitempty"`
	P90     int64 `json:"p90,omitempty"`
	P95     int64 `json:"p95,omitempty"`
	P99     int64 `json:"p99,omitempty"`
}

type PerformanceTestResult struct {
	Config        PerformanceTestConfig    `json:"config"`
	Failures      map[string]int32         `json:"failures,omitempty"`
	NumPending    int                      `json:"numPending,omitempty"`
	RequestID     string                   `json:"requestId,omitempty"`
	ResponseTimes PerformanceResponseTimes `json:"responseTimes,omitempty"`
	Result        string                   `json:"result"`
	SuccessRate   int32                    `json:"successRate,omitempty"`
	Target        string                   `json:"target,omitempty"`
}

// MutationResult is the stable CLI response for an empty successful mutation.
type MutationResult struct {
	Resource    string      `json:"resource"`
	Identifier  string      `json:"identifier,omitempty"`
	Action      string      `json:"action"`
	Environment Environment `json:"environment"`
	Success     bool        `json:"success"`
}
