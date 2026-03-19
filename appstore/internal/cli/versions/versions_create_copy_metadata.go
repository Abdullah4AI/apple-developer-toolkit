package versions

import (
	"context"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

func normalizeVersionMetadataCopyFields(value, flagName string) ([]string, error) {
	return shared.NormalizeVersionMetadataCopyFields(value, flagName)
}

func resolveVersionMetadataCopyFields(copyFields, excludeFields []string) ([]string, error) {
	return shared.ResolveVersionMetadataCopyFields(copyFields, excludeFields)
}

func copyVersionMetadataFromSource(
	ctx context.Context,
	client *asc.Client,
	appID string,
	platform string,
	sourceVersionString string,
	destinationVersionID string,
	selectedFields []string,
) (*asc.AppStoreVersionMetadataCopySummary, error) {
	return shared.CopyVersionMetadataFromSource(ctx, client, shared.VersionMetadataCopyOptions{
		AppID:                appID,
		Platform:             platform,
		SourceVersion:        sourceVersionString,
		DestinationVersionID: destinationVersionID,
		SelectedFields:       selectedFields,
	})
}
