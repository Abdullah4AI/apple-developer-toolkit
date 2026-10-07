package asc

import "fmt"

// AssetLibraryMutationResult reports an accepted asset lifecycle operation.
type AssetLibraryMutationResult struct {
	AssetID              string `json:"assetId"`
	AssetType            string `json:"assetType"`
	Action               string `json:"action"`
	ReferenceName        string `json:"referenceName,omitempty"`
	Archived             *bool  `json:"archived,omitempty"`
	PreviewFrameTimeCode string `json:"previewFrameTimeCode,omitempty"`
}

func assetLibraryMutationRows(result *AssetLibraryMutationResult) ([]string, [][]string) {
	archived := ""
	if result.Archived != nil {
		archived = fmt.Sprint(*result.Archived)
	}
	return []string{"Asset ID", "Asset Type", "Action", "Reference Name", "Archived", "Poster Timecode"}, [][]string{{result.AssetID, result.AssetType, result.Action, result.ReferenceName, archived, result.PreviewFrameTimeCode}}
}
