package metadata

import (
	"context"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

// fetchMetadataLocalizations overlaps independent collection reads while keeping
// each collection's pagination serial. App-info errors retain precedence; when
// that read fails, cancel and join the version read before returning.
func fetchMetadataLocalizations(ctx context.Context, client *asc.Client, appInfoID, versionID string, needAppInfo, needVersion bool) ([]asc.Resource[asc.AppInfoLocalizationAttributes], []asc.Resource[asc.AppStoreVersionLocalizationAttributes], error) {
	if !needAppInfo {
		if !needVersion {
			return nil, nil, nil
		}
		version, err := fetchVersionLocalizations(ctx, client, versionID)
		return nil, version, err
	}
	if !needVersion {
		appInfo, err := fetchAppInfoLocalizations(ctx, client, appInfoID)
		return appInfo, nil, err
	}

	readCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type versionResult struct {
		items []asc.Resource[asc.AppStoreVersionLocalizationAttributes]
		err   error
	}
	versionDone := make(chan versionResult, 1)
	go func() {
		items, err := fetchVersionLocalizations(readCtx, client, versionID)
		versionDone <- versionResult{items: items, err: err}
	}()
	appInfo, appInfoErr := fetchAppInfoLocalizations(readCtx, client, appInfoID)
	if appInfoErr != nil {
		cancel()
	}
	version := <-versionDone
	if appInfoErr != nil {
		return nil, nil, appInfoErr
	}
	if version.err != nil {
		return nil, nil, version.err
	}
	return appInfo, version.items, nil
}
