package asc

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/screenshotcatalog"
)

// App Store Connect asks for an in-app purchase or subscription App Review
// screenshot that "meets any of the screenshot specifications your app
// supports":
// https://developer.apple.com/help/app-store-connect/reference/in-app-purchase-information
//
// The screenshot specifications accept .jpeg, .jpg, and .png files without
// alpha channels or transparency, at the sizes listed per device:
// https://developer.apple.com/help/app-store-connect/reference/app-information/screenshot-specifications
//
// The upload itself succeeds for any file; App Store Connect reports
// IMAGE_BAD_FILE_EXTENSION or IMAGE_INCORRECT_DIMENSIONS only once asset
// delivery fails, leaving a failed screenshot on the product.

// reviewScreenshotExtensions are the file extensions the screenshot
// specifications accept, mapped to the image format each one names.
var reviewScreenshotExtensions = map[string]string{
	".png":  "png",
	".jpg":  "jpeg",
	".jpeg": "jpeg",
}

// reviewScreenshotLegacyDimensions are sizes the screenshot specifications
// still list that the App Store screenshot catalog does not carry, such as
// the 3.5-inch and 4-inch iPhone and 9.7-inch iPad sizes without a status bar.
var reviewScreenshotLegacyDimensions = []ScreenshotDimension{
	{Width: 640, Height: 920},
	{Width: 960, Height: 600},
	{Width: 640, Height: 1096},
	{Width: 1136, Height: 600},
	{Width: 1536, Height: 2008},
	{Width: 2048, Height: 1496},
	{Width: 768, Height: 1004},
	{Width: 1024, Height: 748},
	{Width: 768, Height: 1024},
	{Width: 1024, Height: 768},
}

// reviewScreenshotDimensions returns every documented screenshot size,
// sorted by width then height.
func reviewScreenshotDimensions() []ScreenshotDimension {
	seen := make(map[ScreenshotDimension]struct{})
	dims := make([]ScreenshotDimension, 0, 96)
	add := func(dim ScreenshotDimension) {
		if _, ok := seen[dim]; ok {
			return
		}
		seen[dim] = struct{}{}
		dims = append(dims, dim)
	}
	for _, displayType := range screenshotcatalog.DisplayTypes() {
		catalogDims, _ := screenshotcatalog.Dimensions(displayType)
		for _, dim := range catalogDims {
			add(ScreenshotDimension{Width: dim.Width, Height: dim.Height})
		}
	}
	for _, dim := range reviewScreenshotLegacyDimensions {
		add(dim)
	}
	sort.Slice(dims, func(i, j int) bool {
		if dims[i].Width == dims[j].Width {
			return dims[i].Height < dims[j].Height
		}
		return dims[i].Width < dims[j].Width
	})
	return dims
}

// CheckReviewScreenshotImage reads the image header from source and reports
// whether App Store Connect will accept it as an in-app purchase or
// subscription App Review screenshot. path names the file in messages and
// supplies the extension App Store Connect sees. File format problems are
// errors. Documented constraints that no live upload has confirmed for review
// screenshots, the size list and the alpha rule, are returned as warnings.
func CheckReviewScreenshotImage(path string, source io.Reader) ([]string, error) {
	// Identify the container from its signature rather than from Go's
	// decoders, which reject some valid encodings such as arithmetic-coded
	// JPEG; only the decoder's opinion of the contents is advisory.
	signature := make([]byte, 12)
	n, err := io.ReadFull(source, signature)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read review screenshot %q: %w", path, err)
	}
	signature = signature[:n]
	format := sniffImageFormat(signature)
	switch format {
	case "png", "jpeg":
	case "":
		return nil, fmt.Errorf("review screenshot %q is not a PNG or JPEG image; App Store Connect accepts .png, .jpg, or .jpeg screenshots", path)
	default:
		return nil, fmt.Errorf("review screenshot %q is %s data; App Store Connect accepts PNG or JPEG screenshots (.png, .jpg, or .jpeg); re-export it as PNG or JPEG", path, strings.ToUpper(format))
	}

	extension := strings.ToLower(filepath.Ext(path))
	if _, ok := reviewScreenshotExtensions[extension]; !ok {
		described := "no file extension"
		if extension != "" {
			described = "a " + extension + " extension"
		}
		message := fmt.Sprintf("review screenshot %q has %s; App Store Connect accepts only .png, .jpg, or .jpeg file names", path, described)
		if renamed, ok := SuggestedImageFileName(path, format); ok {
			message += "; rename it to " + renamed
		}
		return nil, errors.New(message)
	}
	if err := ValidateImageFormatMatchesExtension(path, format); err != nil {
		return nil, fmt.Errorf("review screenshot %w", err)
	}

	cfg, _, err := image.DecodeConfig(io.MultiReader(bytes.NewReader(signature), source))
	if err != nil {
		var unsupported jpeg.UnsupportedError
		if format == "jpeg" && errors.As(err, &unsupported) {
			return []string{fmt.Sprintf("review screenshot %q uses a JPEG encoding this tool cannot read (%v), so its size and alpha were not checked; if delivery fails, re-export it as baseline JPEG or PNG.", path, err)}, nil
		}
		return nil, fmt.Errorf("review screenshot %q is not a readable %s image (%w); re-export it as PNG or JPEG", path, strings.ToUpper(format), err)
	}

	var warnings []string
	actual := ScreenshotDimension{Width: cfg.Width, Height: cfg.Height}
	documented := reviewScreenshotDimensions()
	if !acceptsScreenshotDimension(documented, actual) {
		warnings = append(warnings, fmt.Sprintf(
			"review screenshot %q is %s pixels, which matches no documented App Store screenshot size (nearest documented size: %s). Apple asks for a review screenshot that meets a screenshot specification the app supports; if delivery fails with IMAGE_INCORRECT_DIMENSIONS, resize it to a documented size.",
			path,
			actual,
			nearestScreenshotDimension(documented, actual),
		))
	}
	if colorModelHasAlpha(cfg.ColorModel) {
		warnings = append(warnings, fmt.Sprintf("review screenshot %q has an alpha channel or transparency; Apple's screenshot specifications say images can't include alpha channels or transparency. If delivery fails, re-export it without alpha.", path))
	}
	return warnings, nil
}

// reviewScreenshotMaxDecodePixels bounds the full decode. The largest
// documented screenshot is 3840x2160; anything far larger already gets a
// size warning, and decoding it would allocate memory in proportion to its
// declared dimensions rather than its file size.
const reviewScreenshotMaxDecodePixels = 16 * 1024 * 1024

// ReviewScreenshotDecodeWarning fully decodes an image that already passed
// CheckReviewScreenshotImage and returns a warning when the payload is
// truncated or corrupt, or an empty string when it decodes. It warns rather
// than rejects because Go's decoders do not support every encoding App Store
// Connect accepts, such as arithmetic-coded JPEG. Images whose declared size
// exceeds reviewScreenshotMaxDecodePixels are not decoded.
func ReviewScreenshotDecodeWarning(path string, source io.ReaderAt, size int64) string {
	cfg, _, err := image.DecodeConfig(io.NewSectionReader(source, 0, size))
	if err != nil || int64(cfg.Width)*int64(cfg.Height) > reviewScreenshotMaxDecodePixels {
		return ""
	}
	if _, _, err := image.Decode(io.NewSectionReader(source, 0, size)); err != nil {
		return fmt.Sprintf("review screenshot %q could not be fully decoded (%v); if delivery fails, re-export it as PNG or JPEG.", path, err)
	}
	return ""
}

// sniffImageFormat names the image container from its leading bytes, using
// the names the image package registers ("png", "jpeg", "gif"). WebP is named
// so its error is specific; anything else returns an empty string.
func sniffImageFormat(signature []byte) string {
	switch {
	case bytes.HasPrefix(signature, []byte("\x89PNG\r\n\x1a\n")):
		return "png"
	case bytes.HasPrefix(signature, []byte("\xff\xd8\xff")):
		return "jpeg"
	case bytes.HasPrefix(signature, []byte("GIF87a")), bytes.HasPrefix(signature, []byte("GIF89a")):
		return "gif"
	case len(signature) >= 12 && bytes.Equal(signature[0:4], []byte("RIFF")) && bytes.Equal(signature[8:12], []byte("WEBP")):
		return "webp"
	default:
		return ""
	}
}

func acceptsScreenshotDimension(dims []ScreenshotDimension, target ScreenshotDimension) bool {
	for _, dim := range dims {
		if dim == target {
			return true
		}
	}
	return false
}

// nearestScreenshotDimension picks the accepted size closest to actual,
// preferring the same orientation so a resize does not rotate the content.
func nearestScreenshotDimension(accepted []ScreenshotDimension, actual ScreenshotDimension) ScreenshotDimension {
	orientation := func(dim ScreenshotDimension) int {
		switch {
		case dim.Height > dim.Width:
			return 1
		case dim.Width > dim.Height:
			return -1
		default:
			return 0
		}
	}
	distance := func(dim ScreenshotDimension) int {
		return absInt(dim.Width-actual.Width) + absInt(dim.Height-actual.Height)
	}

	var best ScreenshotDimension
	bestDistance := -1
	for _, dim := range accepted {
		if actualOrientation := orientation(actual); actualOrientation != 0 && orientation(dim) != actualOrientation {
			continue
		}
		if d := distance(dim); bestDistance < 0 || d < bestDistance {
			best, bestDistance = dim, d
		}
	}
	return best
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// colorModelHasAlpha reports whether a decoded header declares alpha. The PNG
// decoder reports NRGBA models only for color types that carry an alpha
// channel (opaque true-color PNGs report RGBA models), and folds a tRNS chunk
// into the palette of an indexed PNG.
func colorModelHasAlpha(model color.Model) bool {
	if palette, ok := model.(color.Palette); ok {
		for _, entry := range palette {
			if _, _, _, alpha := entry.RGBA(); alpha != 0xffff {
				return true
			}
		}
		return false
	}
	return model == color.NRGBAModel || model == color.NRGBA64Model
}
