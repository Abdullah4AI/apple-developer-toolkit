package migrate

import (
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/assets"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/storeassets"
)

type (
	AppClipLayout = storeassets.AppClipLayout
	PreviewLayout = storeassets.PreviewLayout
)

func readAppClipLayout(dir string) (AppClipLayout, bool, error) { return storeassets.ReadAppClip(dir) }

func readPreviewLayout(dir string) ([]PreviewLayout, error) { return storeassets.ReadPreviews(dir) }

func validPosterFrame(value string) bool { return assets.ValidPreviewFrameTimeCode(value) }
