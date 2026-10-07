package asc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type assetLibraryResource struct {
	ID         string                     `json:"id"`
	Type       string                     `json:"type"`
	Attributes map[string]json.RawMessage `json:"attributes"`
}

// ValidateAssetLibraryResponse verifies the resource envelope without imposing
// a schema on undocumented attribute values.
func ValidateAssetLibraryResponse(payload []byte) error {
	_, err := assetLibraryResources(payload)
	return err
}

func assetLibraryResources(payload []byte) ([]assetLibraryResource, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("asset-library: invalid response: %w", err)
	}
	data := bytes.TrimSpace(envelope.Data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil, fmt.Errorf("asset-library: response has no resource data")
	}
	var items []assetLibraryResource
	switch data[0] {
	case '[':
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, fmt.Errorf("asset-library: invalid collection: %w", err)
		}
	case '{':
		var item assetLibraryResource
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, fmt.Errorf("asset-library: invalid resource: %w", err)
		}
		items = []assetLibraryResource{item}
	default:
		return nil, fmt.Errorf("asset-library: resource data must be an object or array")
	}
	for _, item := range items {
		if item.ID == "" || item.Type == "" {
			return nil, fmt.Errorf("asset-library: resource missing id or type")
		}
	}
	return items, nil
}

// AssetLibraryRows validates a live Asset Library envelope and derives display
// rows without re-encoding or filtering the caller's JSON response.
func AssetLibraryRows(payload []byte) ([]string, [][]string, error) {
	items, err := assetLibraryResources(payload)
	if err != nil {
		return nil, nil, err
	}
	headers := []string{"ID", "Name", "Category", "State", "Placement Type"}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		if item.ID == "" || item.Type == "" {
			return nil, nil, fmt.Errorf("asset-library: resource missing id or type")
		}
		if item.Type == "appAssetLibraryRefData" {
			headers = []string{"Media", "Spec ID", "Dimensions", "Aspect Ratio", "Placements"}
			for _, media := range []string{"image", "video"} {
				var specs []struct {
					ID         string          `json:"specId"`
					Ratio      json.RawMessage `json:"aspectRatio"`
					Placements []string        `json:"compatiblePlacementTypes"`
					Dimensions struct {
						MinWidth  int `json:"minWidth"`
						MaxWidth  int `json:"maxWidth"`
						MinHeight int `json:"minHeight"`
						MaxHeight int `json:"maxHeight"`
					} `json:"dimensions"`
				}
				if raw, ok := item.Attributes[media+"Specs"]; ok {
					if err := json.Unmarshal(raw, &specs); err != nil {
						return nil, nil, fmt.Errorf("asset-library: invalid %s specs: %w", media, err)
					}
				}
				for _, spec := range specs {
					d := spec.Dimensions
					var ratio string
					if err := json.Unmarshal(spec.Ratio, &ratio); err != nil && len(spec.Ratio) > 0 {
						var compact bytes.Buffer
						if err := json.Compact(&compact, spec.Ratio); err != nil {
							return nil, nil, fmt.Errorf("asset-library: invalid aspect ratio: %w", err)
						}
						ratio = compact.String()
					}
					rows = append(rows, []string{media, spec.ID, fmt.Sprintf("%d-%d x %d-%d", d.MinWidth, d.MaxWidth, d.MinHeight, d.MaxHeight), ratio, strings.Join(spec.Placements, ", ")})
				}
			}
			continue
		}
		text := func(key string) string {
			var value string
			_ = json.Unmarshal(item.Attributes[key], &value)
			return value
		}
		name := text("referenceName")
		if name == "" {
			name = text("fileName")
		}
		rows = append(rows, []string{item.ID, name, text("category"), text("state"), text("placementType")})
	}
	return headers, rows, nil
}

// AssetLibraryImageUploadResult reports upload and processing independently.
// ImageID is retained in a partial result after reservation.
type AssetLibraryImageUploadResult struct {
	ImageID   string `json:"imageId"`
	Category  string `json:"category,omitempty"`
	LibraryID string `json:"libraryId"`
	FileName  string `json:"fileName"`
	FileSize  int64  `json:"fileSize"`
	Uploaded  bool   `json:"uploaded"`
	Ready     bool   `json:"ready"`
	State     string `json:"state,omitempty"`
	SpecID    string `json:"specId,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
}

func assetLibraryImageUploadRows(result *AssetLibraryImageUploadResult) ([]string, [][]string) {
	return []string{"Image ID", "Library ID", "File", "Uploaded", "Ready", "State", "Dimensions"}, [][]string{{result.ImageID, result.LibraryID, result.FileName, fmt.Sprint(result.Uploaded), fmt.Sprint(result.Ready), result.State, fmt.Sprintf("%d x %d", result.Width, result.Height)}}
}

// AssetLibraryVideoUploadResult reports upload and processing independently.
// VideoID is retained in a partial result after reservation.
type AssetLibraryVideoUploadResult struct {
	VideoID   string `json:"videoId"`
	Category  string `json:"category,omitempty"`
	LibraryID string `json:"libraryId"`
	FileName  string `json:"fileName"`
	FileSize  int64  `json:"fileSize"`
	Uploaded  bool   `json:"uploaded"`
	Ready     bool   `json:"ready"`
	State     string `json:"state,omitempty"`
	SpecID    string `json:"specId,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
}

func assetLibraryVideoUploadRows(result *AssetLibraryVideoUploadResult) ([]string, [][]string) {
	return []string{"Video ID", "Library ID", "File", "Uploaded", "Ready", "State", "Dimensions"}, [][]string{{result.VideoID, result.LibraryID, result.FileName, fmt.Sprint(result.Uploaded), fmt.Sprint(result.Ready), result.State, fmt.Sprintf("%d x %d", result.Width, result.Height)}}
}
