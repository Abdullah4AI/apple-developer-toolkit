package productpages

import (
	"context"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

func screenshotSetListResult(ctx context.Context, client *asc.Client, localizationID string, response *asc.AppScreenshotSetsResponse) (*asc.AppScreenshotSetListResult, error) {
	sets, err := client.AppScreenshotSetsWithScreenshots(ctx, response, shared.ContextWithTimeout)
	if err != nil {
		return nil, err
	}
	return &asc.AppScreenshotSetListResult{LocalizationID: localizationID, Sets: sets}, nil
}
