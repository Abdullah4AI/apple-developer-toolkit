package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

var assetLibraryProcessingPollInterval = 2 * time.Second

type assetLibraryImageUploadResponse struct {
	Data struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			State            string            `json:"state"`
			SpecID           string            `json:"specId"`
			UploadOperations []UploadOperation `json:"uploadOperations"`
			ImageAsset       *struct {
				Width  int `json:"width"`
				Height int `json:"height"`
			} `json:"imageAsset"`
		} `json:"attributes"`
	} `json:"data"`
}

// UploadAssetLibraryImage reserves, uploads, commits, and waits for an image.
// A partial result retains the reserved ID when a later operation fails.
// The caller validates the image and owns the overall upload timeout.
func (c *Client) UploadAssetLibraryImage(ctx context.Context, libraryID, fileName string, file *os.File, fileSize int64, requestContext RequestContextFunc) (AssetLibraryImageUploadResult, error) {
	return c.UploadAssetLibraryImageWithCategory(ctx, libraryID, fileName, file, fileSize, "CREATIVE_ASSETS", requestContext)
}

// UploadAssetLibraryImageWithCategory uploads an independently managed image.
func (c *Client) UploadAssetLibraryImageWithCategory(ctx context.Context, libraryID, fileName string, file *os.File, fileSize int64, category string, requestContext RequestContextFunc) (AssetLibraryImageUploadResult, error) {
	result := AssetLibraryImageUploadResult{LibraryID: libraryID, FileName: fileName, FileSize: fileSize, Category: category}
	if category != "CREATIVE_ASSETS" && category != "APP_SCREENSHOTS_AND_PREVIEWS" {
		return result, fmt.Errorf("unsupported asset category %q", category)
	}
	if file == nil {
		return result, fmt.Errorf("image file is required")
	}
	if strings.TrimSpace(libraryID) == "" {
		return result, fmt.Errorf("library ID is required")
	}
	payload := struct {
		Data struct {
			Type       string `json:"type"`
			Attributes struct {
				FileSize int64  `json:"fileSize"`
				FileName string `json:"fileName"`
				Category string `json:"category"`
			} `json:"attributes"`
			Relationships struct {
				AssetLibrary Relationship `json:"assetLibrary"`
			} `json:"relationships"`
		} `json:"data"`
	}{}
	payload.Data.Type = "appAssetLibraryImages"
	payload.Data.Attributes.FileSize, payload.Data.Attributes.FileName, payload.Data.Attributes.Category = fileSize, fileName, category
	payload.Data.Relationships.AssetLibrary.Data = ResourceData{Type: "appAssetLibraries", ID: libraryID}
	body, err := json.Marshal(payload)
	if err != nil {
		return result, err
	}
	request := func(method, path string, body []byte) ([]byte, error) {
		requestCtx, cancel := requestContextFor(ctx, requestContext)
		defer cancel()
		return c.RawRequest(requestCtx, method, path, body)
	}
	raw, err := request("POST", "/v1/appAssetLibraryImages", body)
	if err != nil {
		return result, fmt.Errorf("reserve image: %w", err)
	}
	var reserved assetLibraryImageUploadResponse
	decodeErr := json.Unmarshal(raw, &reserved)
	result.ImageID = reserved.Data.ID
	result.State = reserved.Data.Attributes.State
	if decodeErr != nil {
		return result, fmt.Errorf("parse image reservation: %w", decodeErr)
	}
	if result.ImageID == "" || reserved.Data.Type != "appAssetLibraryImages" {
		return result, fmt.Errorf("image reservation missing image resource identity")
	}
	if err := UploadAssetFromFile(ctx, file, fileSize, reserved.Data.Attributes.UploadOperations); err != nil {
		return result, fmt.Errorf("upload image %s: %w", result.ImageID, err)
	}
	path := "/v1/appAssetLibraryImages/" + url.PathEscape(result.ImageID)
	commit := struct {
		Data struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				Uploaded bool `json:"uploaded"`
			} `json:"attributes"`
		} `json:"data"`
	}{}
	commit.Data.Type, commit.Data.ID, commit.Data.Attributes.Uploaded = "appAssetLibraryImages", result.ImageID, true
	body, err = json.Marshal(commit)
	if err != nil {
		return result, err
	}
	if _, err := request("PATCH", path, body); err != nil {
		return result, fmt.Errorf("commit image %s: %w", result.ImageID, err)
	}
	result.Uploaded = true
	_, err = PollUntilTolerant(ctx, assetLibraryProcessingPollInterval, func(context.Context) (struct{}, bool, error) {
		raw, err := request("GET", path, nil)
		if err != nil {
			return struct{}{}, false, err
		}
		var current assetLibraryImageUploadResponse
		if err := json.Unmarshal(raw, &current); err != nil {
			return struct{}{}, false, fmt.Errorf("parse processed image: %w", err)
		}
		if current.Data.ID != result.ImageID || current.Data.Type != "appAssetLibraryImages" {
			return struct{}{}, false, fmt.Errorf("processing response does not identify reserved image")
		}
		attrs := current.Data.Attributes
		result.State, result.SpecID = attrs.State, attrs.SpecID
		if attrs.State == "FAILED" {
			return struct{}{}, false, fmt.Errorf("image processing reached FAILED")
		}
		if attrs.ImageAsset != nil {
			result.Width, result.Height = attrs.ImageAsset.Width, attrs.ImageAsset.Height
		}
		result.Ready = attrs.State == "PREPARE_FOR_SUBMISSION" && attrs.ImageAsset != nil && attrs.ImageAsset.Width > 0 && attrs.ImageAsset.Height > 0
		return struct{}{}, result.Ready, nil
	}, PollOptions{Tolerate: IsTransientWaitError})
	if err != nil {
		return result, fmt.Errorf("wait for image %s processing (state %q): %w", result.ImageID, result.State, err)
	}
	return result, nil
}
