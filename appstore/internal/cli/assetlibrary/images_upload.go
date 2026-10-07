package assetlibrary

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/rootfs"
)

func imagesUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("images upload", flag.ExitOnError)
	libraryID := shared.BindResourceIDFlag(fs, "library-id", "appAssetLibraries", "Asset Library ID")
	path := fs.String("file", "", "Image file to upload")
	category := fs.String("category", "CREATIVE_ASSETS", "Asset category: CREATIVE_ASSETS or APP_SCREENSHOTS_AND_PREVIEWS")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "upload", ShortUsage: "asc asset-library images upload --library-id ID --file image.png [flags]",
		ShortHelp: "Upload an image to the Asset Library and wait for processing.",
		LongHelp: `Upload an image to the Asset Library without assigning it to a placement or submitting review.

The command waits for processed image data under ASC_UPLOAD_TIMEOUT. On failure
after reservation, it prints a partial receipt with the image ID for inspection
or cleanup. It never automatically deletes the reserved image.

Examples:
  asc asset-library images upload --library-id LIBRARY_ID --file ./header.png`,
		FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 0 {
				return shared.UsageErrorf("asset-library images upload: unexpected argument %q", args[0])
			}
			if err := shared.ValidateBoundOutputFlags(fs); err != nil {
				return shared.UsageErrorf("asset-library images upload: %v", err)
			}
			selectedCategory, categoryErr := normalizeAssetCategory(*category)
			if categoryErr != nil {
				return shared.UsageErrorf("asset-library images upload: %v", categoryErr)
			}
			id := strings.TrimSpace(*libraryID)
			if id == "" {
				return shared.UsageError("asset-library images upload: --library-id is required")
			}
			if !resourceIDPattern.MatchString(id) {
				return shared.UsageError("asset-library images upload: --library-id must be a resource ID")
			}
			filePath := strings.TrimSpace(*path)
			if filePath == "" {
				return shared.UsageError("asset-library images upload: --file is required")
			}
			switch strings.ToLower(filepath.Ext(filePath)) {
			case ".png", ".jpg", ".jpeg":
			default:
				return shared.UsageError("asset-library images upload: --file must have a .png, .jpg, or .jpeg extension")
			}
			file, err := rootfs.OpenFile(filePath)
			if err != nil {
				return shared.UsageErrorf("asset-library images upload: --file: %v", err)
			}
			defer file.Close()
			info, err := file.Stat()
			if err != nil {
				return fmt.Errorf("asset-library images upload: stat file: %w", err)
			}
			if err := asc.ValidateAssetFileInfo(filePath, info); err != nil {
				return shared.UsageErrorf("asset-library images upload: --file: %v", err)
			}
			if info.Size() > 524288000 {
				return shared.UsageError("asset-library images upload: file exceeds 500 MiB")
			}
			format, err := asc.ReadAppStoreImageFormatFrom(file)
			if err != nil {
				return shared.UsageErrorf("asset-library images upload: --file: %v", err)
			}
			if format != "png" && format != "jpeg" {
				return shared.UsageError("asset-library images upload: --file must be a PNG or JPEG image")
			}
			if err := asc.ValidateImageFormatMatchesExtension(filePath, format); err != nil {
				return shared.UsageErrorf("asset-library images upload: --file: %v", err)
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				return fmt.Errorf("asset-library images upload: rewind file: %w", err)
			}
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("asset-library images upload: %w", err)
			}
			uploadCtx, cancel := shared.ContextWithUploadTimeout(ctx)
			defer cancel()
			result, uploadErr := client.UploadAssetLibraryImageWithCategory(uploadCtx, id, filepath.Base(filePath), file, info.Size(), selectedCategory, shared.ContextWithTimeout)
			if uploadErr == nil || result.ImageID != "" {
				if err := shared.PrintOutput(&result, *output.Output, *output.Pretty); err != nil {
					return err
				}
			}
			if uploadErr != nil {
				return fmt.Errorf("asset-library images upload: %w", uploadErr)
			}
			return nil
		},
	}
}
