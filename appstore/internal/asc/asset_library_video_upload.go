package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type assetLibraryVideoUploadResponse struct {
	Data struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			State             string            `json:"state"`
			SpecID            string            `json:"specId"`
			UploadOperations  []UploadOperation `json:"uploadOperations"`
			VideoAsset        string            `json:"videoAsset"`
			PreviewFrameImage *struct {
				Image *struct {
					Width  int `json:"width"`
					Height int `json:"height"`
				} `json:"image"`
			} `json:"previewFrameImage"`
		} `json:"attributes"`
	} `json:"data"`
}

// UploadAssetLibraryVideo reserves, uploads, commits, and waits for a video.
// A partial result retains the reserved ID when a later operation fails.
// The caller validates the video and owns the overall upload timeout.
func (c *Client) UploadAssetLibraryVideo(ctx context.Context, libraryID, fileName string, file *os.File, fileSize int64, requestContext RequestContextFunc) (AssetLibraryVideoUploadResult, error) {
	return c.UploadAssetLibraryVideoWithCategory(ctx, libraryID, fileName, file, fileSize, "CREATIVE_ASSETS", requestContext)
}

// UploadAssetLibraryVideoWithCategory uploads an independently managed video.
func (c *Client) UploadAssetLibraryVideoWithCategory(ctx context.Context, libraryID, fileName string, file *os.File, fileSize int64, category string, requestContext RequestContextFunc) (AssetLibraryVideoUploadResult, error) {
	result := AssetLibraryVideoUploadResult{LibraryID: libraryID, FileName: fileName, FileSize: fileSize, Category: category}
	if category != "CREATIVE_ASSETS" && category != "APP_SCREENSHOTS_AND_PREVIEWS" {
		return result, fmt.Errorf("unsupported asset category %q", category)
	}
	if file == nil {
		return result, fmt.Errorf("video file is required")
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
	payload.Data.Type = "appAssetLibraryVideos"
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
	raw, err := request("POST", "/v1/appAssetLibraryVideos", body)
	if err != nil {
		return result, fmt.Errorf("reserve video: %w", err)
	}
	var reserved assetLibraryVideoUploadResponse
	decodeErr := json.Unmarshal(raw, &reserved)
	result.VideoID = reserved.Data.ID
	result.State = reserved.Data.Attributes.State
	if decodeErr != nil {
		return result, fmt.Errorf("parse video reservation: %w", decodeErr)
	}
	if result.VideoID == "" || reserved.Data.Type != "appAssetLibraryVideos" {
		return result, fmt.Errorf("video reservation missing video resource identity")
	}
	if err := UploadAssetFromFile(ctx, file, fileSize, reserved.Data.Attributes.UploadOperations); err != nil {
		return result, fmt.Errorf("upload video %s: %w", result.VideoID, err)
	}
	path := "/v1/appAssetLibraryVideos/" + url.PathEscape(result.VideoID)
	commit := struct {
		Data struct {
			Type       string `json:"type"`
			ID         string `json:"id"`
			Attributes struct {
				Uploaded bool `json:"uploaded"`
			} `json:"attributes"`
		} `json:"data"`
	}{}
	commit.Data.Type, commit.Data.ID, commit.Data.Attributes.Uploaded = "appAssetLibraryVideos", result.VideoID, true
	body, err = json.Marshal(commit)
	if err != nil {
		return result, err
	}
	if _, err := request("PATCH", path, body); err != nil {
		return result, fmt.Errorf("commit video %s: %w", result.VideoID, err)
	}
	result.Uploaded = true
	_, err = PollUntilTolerant(ctx, assetLibraryProcessingPollInterval, func(context.Context) (struct{}, bool, error) {
		raw, err := request("GET", path, nil)
		if err != nil {
			return struct{}{}, false, err
		}
		var current assetLibraryVideoUploadResponse
		if err := json.Unmarshal(raw, &current); err != nil {
			return struct{}{}, false, fmt.Errorf("parse processed video: %w", err)
		}
		if current.Data.ID != result.VideoID || current.Data.Type != "appAssetLibraryVideos" {
			return struct{}{}, false, fmt.Errorf("processing response does not identify reserved video")
		}
		attrs := current.Data.Attributes
		result.State, result.SpecID = attrs.State, attrs.SpecID
		if attrs.State == "FAILED" {
			return struct{}{}, false, fmt.Errorf("video processing reached FAILED")
		}
		if attrs.PreviewFrameImage != nil && attrs.PreviewFrameImage.Image != nil {
			result.Width, result.Height = attrs.PreviewFrameImage.Image.Width, attrs.PreviewFrameImage.Image.Height
		}
		videoURL, parseErr := url.Parse(attrs.VideoAsset)
		videoAvailable := parseErr == nil && videoURL.Host != "" && (videoURL.Scheme == "https" || videoURL.Scheme == "http")
		result.Ready = attrs.State == "PREPARE_FOR_SUBMISSION" && attrs.SpecID != "" && videoAvailable
		return struct{}{}, result.Ready, nil
	}, PollOptions{Tolerate: IsTransientWaitError})
	if err != nil {
		return result, fmt.Errorf("wait for video %s processing (state %q): %w", result.VideoID, result.State, err)
	}
	return result, nil
}
