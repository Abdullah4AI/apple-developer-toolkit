package shared

import (
	"context"
	"fmt"
	"os"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

// PrepareBuildUpload creates the build upload and file reservation records used
// by publish/build upload flows before the binary is transferred.
func PrepareBuildUpload(ctx context.Context, client *asc.Client, appID string, fileInfo os.FileInfo, version, buildNumber string, platform asc.Platform, uti asc.UTI) (*asc.BuildUploadResponse, *asc.BuildUploadFileResponse, error) {
	uploadReq := asc.BuildUploadCreateRequest{
		Data: asc.BuildUploadCreateData{
			Type: asc.ResourceTypeBuildUploads,
			Attributes: asc.BuildUploadAttributes{
				CFBundleShortVersionString: version,
				CFBundleVersion:            buildNumber,
				Platform:                   platform,
			},
			Relationships: &asc.BuildUploadRelationships{
				App: &asc.Relationship{
					Data: asc.ResourceData{Type: asc.ResourceTypeApps, ID: appID},
				},
			},
		},
	}

	uploadResp, err := client.CreateBuildUpload(ctx, uploadReq)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create upload record: %w", err)
	}

	fileReq := asc.BuildUploadFileCreateRequest{
		Data: asc.BuildUploadFileCreateData{
			Type: asc.ResourceTypeBuildUploadFiles,
			Attributes: asc.BuildUploadFileAttributes{
				FileName:  fileInfo.Name(),
				FileSize:  fileInfo.Size(),
				UTI:       uti,
				AssetType: asc.AssetTypeAsset,
			},
			Relationships: &asc.BuildUploadFileRelationships{
				BuildUpload: &asc.Relationship{
					Data: asc.ResourceData{Type: asc.ResourceTypeBuildUploads, ID: uploadResp.Data.ID},
				},
			},
		},
	}

	fileResp, err := client.CreateBuildUploadFile(ctx, fileReq)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create file reservation: %w", err)
	}

	return uploadResp, fileResp, nil
}

// CommitBuildUploadFile marks a reserved upload file as uploaded and optionally
// persists source-file checksums.
func CommitBuildUploadFile(ctx context.Context, client *asc.Client, fileID string, checksums *asc.Checksums) (*asc.BuildUploadFileResponse, error) {
	uploaded := true
	req := asc.BuildUploadFileUpdateRequest{
		Data: asc.BuildUploadFileUpdateData{
			Type: asc.ResourceTypeBuildUploadFiles,
			ID:   fileID,
			Attributes: &asc.BuildUploadFileUpdateAttributes{
				Uploaded:            &uploaded,
				SourceFileChecksums: checksums,
			},
		},
	}

	resp, err := client.UpdateBuildUploadFile(ctx, fileID, req)
	if err != nil {
		return nil, fmt.Errorf("commit upload file: %w", err)
	}
	return resp, nil
}
