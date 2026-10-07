package storekit

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const messagingPath = "inApps/v1/messaging/"

func (c *Client) UploadImage(ctx context.Context, identifier string, size ImageSize, data []byte) error {
	if err := validateUUID("image identifier", identifier); err != nil {
		return err
	}
	if size != ImageSizeFull && size != ImageSizeBulletPoint {
		return fmt.Errorf("image size must be one of: FULL_SIZE, BULLET_POINT")
	}
	if len(data) == 0 {
		return fmt.Errorf("image data is required")
	}
	path := messagingPath + "image/" + url.PathEscape(strings.TrimSpace(identifier)) + "?imageSize=" + url.QueryEscape(string(size))
	return c.request(ctx, http.MethodPut, path, "image/png", data, nil)
}

func (c *Client) DeleteImage(ctx context.Context, identifier string) error {
	if err := validateUUID("image identifier", identifier); err != nil {
		return err
	}
	return c.request(ctx, http.MethodDelete, messagingPath+"image/"+url.PathEscape(strings.TrimSpace(identifier)), "", nil, nil)
}

func (c *Client) ListImages(ctx context.Context) (*ImageListResponse, error) {
	var response ImageListResponse
	if err := c.request(ctx, http.MethodGet, messagingPath+"image/list", "", nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) UploadMessage(ctx context.Context, identifier string, message Message) error {
	if err := validateUUID("message identifier", identifier); err != nil {
		return err
	}
	if err := ValidateMessage(message); err != nil {
		return err
	}
	body, err := jsonBody(message)
	if err != nil {
		return err
	}
	return c.request(ctx, http.MethodPut, messagingPath+"message/"+url.PathEscape(strings.TrimSpace(identifier)), "application/json", body, nil)
}

func (c *Client) DeleteMessage(ctx context.Context, identifier string) error {
	if err := validateUUID("message identifier", identifier); err != nil {
		return err
	}
	return c.request(ctx, http.MethodDelete, messagingPath+"message/"+url.PathEscape(strings.TrimSpace(identifier)), "", nil, nil)
}

func (c *Client) ListMessages(ctx context.Context) (*MessageListResponse, error) {
	var response MessageListResponse
	if err := c.request(ctx, http.MethodGet, messagingPath+"message/list", "", nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) SetDefault(ctx context.Context, productID, locale, messageIdentifier string) (*DefaultMessageResponse, error) {
	if strings.TrimSpace(productID) == "" {
		return nil, fmt.Errorf("product ID is required")
	}
	if strings.TrimSpace(locale) == "" {
		return nil, fmt.Errorf("locale is required")
	}
	if err := validateUUID("message identifier", messageIdentifier); err != nil {
		return nil, err
	}
	body, err := jsonBody(DefaultMessageResponse{MessageIdentifier: strings.TrimSpace(messageIdentifier)})
	if err != nil {
		return nil, err
	}
	response := DefaultMessageResponse{MessageIdentifier: strings.TrimSpace(messageIdentifier)}
	path := defaultPath(productID, locale)
	if err := c.request(ctx, http.MethodPut, path, "application/json", body, nil); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) GetDefault(ctx context.Context, productID, locale string) (*DefaultMessageResponse, error) {
	if strings.TrimSpace(productID) == "" {
		return nil, fmt.Errorf("product ID is required")
	}
	if strings.TrimSpace(locale) == "" {
		return nil, fmt.Errorf("locale is required")
	}
	var response DefaultMessageResponse
	if err := c.request(ctx, http.MethodGet, defaultPath(productID, locale), "", nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) DeleteDefault(ctx context.Context, productID, locale string) error {
	if strings.TrimSpace(productID) == "" {
		return fmt.Errorf("product ID is required")
	}
	if strings.TrimSpace(locale) == "" {
		return fmt.Errorf("locale is required")
	}
	return c.request(ctx, http.MethodDelete, defaultPath(productID, locale), "", nil, nil)
}

func defaultPath(productID, locale string) string {
	return messagingPath + "default/" + url.PathEscape(strings.TrimSpace(productID)) + "/" + url.PathEscape(strings.TrimSpace(locale))
}

func (c *Client) SetRealtimeURL(ctx context.Context, realtimeURL string) (*RealtimeURLResponse, error) {
	if err := ValidateRealtimeURL(realtimeURL); err != nil {
		return nil, err
	}
	body, err := jsonBody(RealtimeURLResponse{RealtimeURL: strings.TrimSpace(realtimeURL)})
	if err != nil {
		return nil, err
	}
	response := RealtimeURLResponse{RealtimeURL: strings.TrimSpace(realtimeURL)}
	if err := c.request(ctx, http.MethodPut, messagingPath+"realtime/url", "application/json", body, nil); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) GetRealtimeURL(ctx context.Context) (*RealtimeURLResponse, error) {
	var response RealtimeURLResponse
	if err := c.request(ctx, http.MethodGet, messagingPath+"realtime/url", "", nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) DeleteRealtimeURL(ctx context.Context) error {
	return c.request(ctx, http.MethodDelete, messagingPath+"realtime/url", "", nil, nil)
}

func (c *Client) StartPerformanceTest(ctx context.Context, originalTransactionID string) (*PerformanceTestStartResponse, error) {
	if strings.TrimSpace(originalTransactionID) == "" {
		return nil, fmt.Errorf("original transaction ID is required")
	}
	body, err := jsonBody(map[string]string{"originalTransactionId": strings.TrimSpace(originalTransactionID)})
	if err != nil {
		return nil, err
	}
	var response PerformanceTestStartResponse
	if err := c.request(ctx, http.MethodPost, messagingPath+"performanceTest", "application/json", body, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) GetPerformanceTestResult(ctx context.Context, requestID string) (*PerformanceTestResult, error) {
	if strings.TrimSpace(requestID) == "" {
		return nil, fmt.Errorf("request ID is required")
	}
	var response PerformanceTestResult
	path := messagingPath + "performanceTest/result/" + url.PathEscape(strings.TrimSpace(requestID))
	if err := c.request(ctx, http.MethodGet, path, "", nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// ValidateMessage checks documented Retention Messaging limits before upload.
func ValidateMessage(message Message) error {
	if strings.TrimSpace(message.Header) == "" {
		return fmt.Errorf("message header is required")
	}
	if utf8.RuneCountInString(message.Header) > 66 {
		return fmt.Errorf("message header must be at most 66 characters")
	}
	if strings.TrimSpace(message.Body) == "" {
		return fmt.Errorf("message body is required")
	}
	if utf8.RuneCountInString(message.Body) > 144 {
		return fmt.Errorf("message body must be at most 144 characters")
	}
	if message.HeaderPosition != "" && message.HeaderPosition != HeaderAboveBody && message.HeaderPosition != HeaderAboveImage {
		return fmt.Errorf("header position must be one of: ABOVE_BODY, ABOVE_IMAGE")
	}
	if message.HeaderPosition == HeaderAboveImage && message.Image == nil {
		return fmt.Errorf("header position ABOVE_IMAGE requires an image")
	}
	if message.Image != nil {
		if err := validateUUID("image identifier", message.Image.ImageIdentifier); err != nil {
			return err
		}
		if strings.TrimSpace(message.Image.AltText) == "" {
			return fmt.Errorf("image alt text is required")
		}
		if utf8.RuneCountInString(message.Image.AltText) > 150 {
			return fmt.Errorf("image alt text must be at most 150 characters")
		}
	}
	if len(message.BulletPoints) > 5 {
		return fmt.Errorf("message must contain at most 5 bullet points")
	}
	for i, bullet := range message.BulletPoints {
		if strings.TrimSpace(bullet.Text) == "" {
			return fmt.Errorf("bullet point %d text is required", i+1)
		}
		if utf8.RuneCountInString(bullet.Text) > 66 {
			return fmt.Errorf("bullet point %d text must be at most 66 characters", i+1)
		}
		if err := validateUUID(fmt.Sprintf("bullet point %d image identifier", i+1), bullet.ImageIdentifier); err != nil {
			return err
		}
		if strings.TrimSpace(bullet.AltText) == "" {
			return fmt.Errorf("bullet point %d alt text is required", i+1)
		}
		if utf8.RuneCountInString(bullet.AltText) > 150 {
			return fmt.Errorf("bullet point %d alt text must be at most 150 characters", i+1)
		}
	}
	return nil
}

func ValidateRealtimeURL(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("realtime URL is required")
	}
	if utf8.RuneCountInString(value) > 256 {
		return fmt.Errorf("realtime URL must be at most 256 characters")
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("realtime URL must be an absolute HTTPS URL")
	}
	return nil
}

func validateUUID(label, value string) error {
	if _, err := uuid.Parse(strings.TrimSpace(value)); err != nil {
		return fmt.Errorf("%s must be a UUID", label)
	}
	return nil
}
