package asc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func encodeOpaquePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, width, height))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func encodeTranslucentPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 128})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func encodeJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, width, height))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func encodeGIF(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewPaletted(image.Rect(0, 0, width, height), color.Palette{color.Black, color.White})
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode gif: %v", err)
	}
	return buf.Bytes()
}

func TestCheckReviewScreenshotImageAcceptsDocumentedScreenshotSizes(t *testing.T) {
	tests := []struct {
		name string
		path string
		data func(t *testing.T) []byte
	}{
		{name: "iPhone 6.9 portrait PNG", path: "review.png", data: func(t *testing.T) []byte { return encodeOpaquePNG(t, 1290, 2796) }},
		{name: "iPhone 6.9 landscape PNG", path: "review.png", data: func(t *testing.T) []byte { return encodeOpaquePNG(t, 2796, 1290) }},
		{name: "iPhone 6.5 JPEG with .jpg", path: "review.jpg", data: func(t *testing.T) []byte { return encodeJPEG(t, 1242, 2688) }},
		{name: "iPhone 6.3 JPEG with uppercase .JPEG", path: "review.JPEG", data: func(t *testing.T) []byte { return encodeJPEG(t, 1206, 2622) }},
		{name: "iPad 13 PNG", path: "review.png", data: func(t *testing.T) []byte { return encodeOpaquePNG(t, 2064, 2752) }},
		{name: "Mac PNG", path: "review.png", data: func(t *testing.T) []byte { return encodeOpaquePNG(t, 2880, 1800) }},
		{name: "iPhone 3.5 without status bar", path: "review.png", data: func(t *testing.T) []byte { return encodeOpaquePNG(t, 640, 920) }},
		{name: "iPad 9.7 without status bar", path: "review.png", data: func(t *testing.T) []byte { return encodeOpaquePNG(t, 768, 1004) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings, err := CheckReviewScreenshotImage(tt.path, bytes.NewReader(tt.data(t)))
			if err != nil {
				t.Fatalf("CheckReviewScreenshotImage() error = %v", err)
			}
			if len(warnings) != 0 {
				t.Fatalf("CheckReviewScreenshotImage() warnings = %q, want none", warnings)
			}
		})
	}
}

func TestCheckReviewScreenshotImageWarnsAboutUndocumentedDimensions(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		width   int
		height  int
		nearest string
	}{
		{name: "portrait near iPhone 6.3", path: "./paywall.png", width: 1179, height: 2560, nearest: "1179x2556"},
		{name: "landscape near iPhone 6.9", path: "wide.png", width: 2800, height: 1290, nearest: "2796x1290"},
		{name: "square", path: "icon.png", width: 1024, height: 1024},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings, err := CheckReviewScreenshotImage(tt.path, bytes.NewReader(encodeOpaquePNG(t, tt.width, tt.height)))
			if err != nil {
				t.Fatalf("CheckReviewScreenshotImage() error = %v, want a warning only", err)
			}
			if len(warnings) != 1 {
				t.Fatalf("warnings = %q, want one size warning", warnings)
			}
			for _, want := range []string{
				fmt.Sprintf("review screenshot %q is %dx%d pixels, which matches no documented App Store screenshot size", tt.path, tt.width, tt.height),
				"IMAGE_INCORRECT_DIMENSIONS",
			} {
				if !strings.Contains(warnings[0], want) {
					t.Fatalf("warning %q does not contain %q", warnings[0], want)
				}
			}
			if tt.nearest != "" && !strings.Contains(warnings[0], "nearest documented size: "+tt.nearest) {
				t.Fatalf("warning %q does not name nearest size %s", warnings[0], tt.nearest)
			}
		})
	}
}

func TestCheckReviewScreenshotImageRejectsUnsupportedFormats(t *testing.T) {
	tests := []struct {
		name string
		path string
		data func(t *testing.T) []byte
		want []string
	}{
		{
			name: "GIF data",
			path: "review.gif",
			data: func(t *testing.T) []byte { return encodeGIF(t, 1290, 2796) },
			want: []string{`review screenshot "review.gif" is GIF data`, "PNG or JPEG"},
		},
		{
			name: "WebP data",
			path: "review.webp",
			data: func(t *testing.T) []byte { return []byte("RIFF\x00\x00\x00\x00WEBPVP8 ") },
			want: []string{`review screenshot "review.webp" is WEBP data`, "PNG or JPEG"},
		},
		{
			name: "HEIC data named .png",
			path: "review.png",
			data: func(t *testing.T) []byte { return []byte("\x00\x00\x00\x18ftypheic\x00\x00\x00\x00") },
			want: []string{`review screenshot "review.png" is not a PNG or JPEG image`, ".png, .jpg, or .jpeg"},
		},
		{
			name: "empty file",
			path: "review.png",
			data: func(t *testing.T) []byte { return nil },
			want: []string{`review screenshot "review.png" is not a PNG or JPEG image`},
		},
		{
			name: "PNG signature without a header",
			path: "review.png",
			data: func(t *testing.T) []byte { return []byte("\x89PNG\r\n\x1a\n") },
			want: []string{`review screenshot "review.png" is not a readable PNG image`},
		},
		{
			name: "JPEG signature with a corrupt header",
			path: "review.jpg",
			data: func(t *testing.T) []byte { return []byte("\xff\xd8\xff\x00garbage") },
			want: []string{`review screenshot "review.jpg" is not a readable JPEG image`},
		},
		{
			name: "JPEG data with .png extension",
			path: "review.png",
			data: func(t *testing.T) []byte { return encodeJPEG(t, 1290, 2796) },
			want: []string{`review screenshot "review.png" is JPEG data but has a .png extension`, "rename it to review.jpg"},
		},
		{
			name: "PNG data without an accepted extension",
			path: "review.img",
			data: func(t *testing.T) []byte { return encodeOpaquePNG(t, 1290, 2796) },
			want: []string{`review screenshot "review.img" has a .img extension`, ".png, .jpg, or .jpeg", "rename it to review.png"},
		},
		{
			name: "PNG data without an extension",
			path: "review",
			data: func(t *testing.T) []byte { return encodeOpaquePNG(t, 1290, 2796) },
			want: []string{`review screenshot "review" has no file extension`, "rename it to review.png"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CheckReviewScreenshotImage(tt.path, bytes.NewReader(tt.data(t)))
			if err == nil {
				t.Fatal("expected format error")
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error %q does not contain %q", err.Error(), want)
				}
			}
		})
	}
}

func encodePNGImage(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestCheckReviewScreenshotImageDoesNotWarnForOpaqueColorPNGs(t *testing.T) {
	opaqueRGB := image.NewRGBA(image.Rect(0, 0, 1290, 2796))
	for i := 3; i < len(opaqueRGB.Pix); i += 4 {
		opaqueRGB.Pix[i] = 0xff
	}
	opaqueRGB64 := image.NewRGBA64(image.Rect(0, 0, 640, 920))
	for i := 6; i < len(opaqueRGB64.Pix); i += 8 {
		opaqueRGB64.Pix[i], opaqueRGB64.Pix[i+1] = 0xff, 0xff
	}
	opaquePalette := image.NewPaletted(image.Rect(0, 0, 640, 920), color.Palette{color.Black, color.White})

	for name, img := range map[string]image.Image{
		"8-bit RGB":      opaqueRGB,
		"16-bit RGB":     opaqueRGB64,
		"opaque palette": opaquePalette,
	} {
		t.Run(name, func(t *testing.T) {
			warnings, err := CheckReviewScreenshotImage("review.png", bytes.NewReader(encodePNGImage(t, img)))
			if err != nil {
				t.Fatalf("CheckReviewScreenshotImage() error = %v", err)
			}
			if len(warnings) != 0 {
				t.Fatalf("warnings = %q, want none for an opaque PNG", warnings)
			}
		})
	}
}

func TestCheckReviewScreenshotImageWarnsAboutTransparentPalette(t *testing.T) {
	img := image.NewPaletted(image.Rect(0, 0, 640, 920), color.Palette{color.Black, color.Transparent})
	warnings, err := CheckReviewScreenshotImage("review.png", bytes.NewReader(encodePNGImage(t, img)))
	if err != nil {
		t.Fatalf("CheckReviewScreenshotImage() error = %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "has an alpha channel or transparency") {
		t.Fatalf("warnings = %q, want one transparency warning", warnings)
	}
}

func TestCheckReviewScreenshotImageWarnsAboutAlphaChannel(t *testing.T) {
	warnings, err := CheckReviewScreenshotImage("review.png", bytes.NewReader(encodeTranslucentPNG(t, 1290, 2796)))
	if err != nil {
		t.Fatalf("CheckReviewScreenshotImage() error = %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %q, want one alpha warning", warnings)
	}
	if !strings.Contains(warnings[0], `review screenshot "review.png" has an alpha channel`) {
		t.Fatalf("unexpected warning %q", warnings[0])
	}
}

func TestCheckReviewScreenshotImageWarnsWhenAJPEGHeaderIsUnsupported(t *testing.T) {
	// SOI followed by an arithmetic-coding SOF9 frame, which App Store Connect
	// can accept but Go's JPEG decoder does not support.
	data := []byte("\xff\xd8\xff\xc9\x00\x0b\x08\x0b\x40\x05\x0a\x01\x01\x11\x00")
	warnings, err := CheckReviewScreenshotImage("review.jpg", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("CheckReviewScreenshotImage() error = %v, want a warning only", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], `review screenshot "review.jpg" uses a JPEG encoding this tool cannot read`) {
		t.Fatalf("warnings = %q, want one unsupported-encoding warning", warnings)
	}
	if warning := ReviewScreenshotDecodeWarning("review.jpg", bytes.NewReader(data), int64(len(data))); warning != "" {
		t.Fatalf("decode warning = %q, want none after the header warning", warning)
	}
}

// pngHeaderOnly returns a PNG signature and IHDR chunk declaring the given
// size with no image data, so nothing but the header is ever allocated.
func pngHeaderOnly(width, height uint32) []byte {
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:4], width)
	binary.BigEndian.PutUint32(ihdr[4:8], height)
	ihdr[8] = 8 // bit depth
	ihdr[9] = 2 // true color
	chunk := append([]byte("IHDR"), ihdr...)
	var buf bytes.Buffer
	buf.WriteString("\x89PNG\r\n\x1a\n")
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(ihdr)))
	buf.Write(chunk)
	_ = binary.Write(&buf, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return buf.Bytes()
}

func TestReviewScreenshotDecodeWarningSkipsOversizedImages(t *testing.T) {
	data := pngHeaderOnly(30000, 30000)
	warnings, err := CheckReviewScreenshotImage("huge.png", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("header check error = %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "is 30000x30000 pixels") {
		t.Fatalf("warnings = %q, want the size warning", warnings)
	}
	if warning := ReviewScreenshotDecodeWarning("huge.png", bytes.NewReader(data), int64(len(data))); warning != "" {
		t.Fatalf("warning = %q, want the oversized image left undecoded", warning)
	}
}

func TestReviewScreenshotDecodeWarningFlagsTruncatedPayload(t *testing.T) {
	data := encodeOpaquePNG(t, 640, 920)
	truncated := data[:len(data)/2]

	if _, err := CheckReviewScreenshotImage("review.png", bytes.NewReader(truncated)); err != nil {
		t.Fatalf("header check error = %v, want the header to pass", err)
	}
	warning := ReviewScreenshotDecodeWarning("review.png", bytes.NewReader(truncated), int64(len(truncated)))
	if !strings.Contains(warning, `review screenshot "review.png" could not be fully decoded`) {
		t.Fatalf("warning = %q, want a decode warning", warning)
	}
	if warning := ReviewScreenshotDecodeWarning("review.png", bytes.NewReader(data), int64(len(data))); warning != "" {
		t.Fatalf("warning = %q, want none for a complete image", warning)
	}
}
