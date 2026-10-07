package asc

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

type declaredAlphaImage struct{ image.Image }

func (declaredAlphaImage) Opaque() bool { return false }

func TestAppStoreImageFormatAlphaMetadata(t *testing.T) {
	opaque := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for i := 0; i < len(opaque.Pix); i += 4 {
		opaque.Pix[i+3] = 255
	}
	palette := image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.NRGBA{A: 0}, color.White})
	for _, tc := range []struct {
		name         string
		img          image.Image
		transparency bool
		wantAlpha    bool
		jpeg         bool
	}{
		{name: "RGB PNG", img: opaque},
		{name: "JPEG", img: opaque, jpeg: true},
		{name: "opaque alpha channel", img: declaredAlphaImage{opaque}, wantAlpha: true},
		{name: "transparent alpha channel", img: image.NewNRGBA(image.Rect(0, 0, 2, 2)), wantAlpha: true},
		{name: "indexed transparency", img: palette, wantAlpha: true},
		{name: "truecolor transparency", img: opaque, transparency: true, wantAlpha: true},
		{name: "grayscale transparency", img: image.NewGray(image.Rect(0, 0, 2, 2)), transparency: true, wantAlpha: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var encoded bytes.Buffer
			if tc.jpeg {
				if err := jpeg.Encode(&encoded, tc.img, nil); err != nil {
					t.Fatal(err)
				}
			} else if err := png.Encode(&encoded, tc.img); err != nil {
				t.Fatal(err)
			}
			content := encoded.Bytes()
			if tc.transparency {
				length := 6
				if _, ok := tc.img.(*image.Gray); ok {
					length = 2
				}
				chunk := make([]byte, length+12)
				binary.BigEndian.PutUint32(chunk, uint32(length))
				copy(chunk[4:8], "tRNS")
				binary.BigEndian.PutUint32(chunk[8+length:], crc32.ChecksumIEEE(chunk[4:8+length]))
				content = append(append(append([]byte{}, content[:33]...), chunk...), content[33:]...)
			}
			format, err := ReadAppStoreImageFormatFrom(bytes.NewReader(content))
			if tc.wantAlpha {
				if err == nil || !strings.Contains(err.Error(), "alpha channel or transparency") {
					t.Fatalf("format %q error %v", format, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Count scanner reads after its rewind, independently of format sniffing.
type pngMetadataReadCounter struct {
	*bytes.Reader
	readBytes int
}

func (r *pngMetadataReadCounter) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.readBytes += n
	return n, err
}

func (r *pngMetadataReadCounter) Seek(offset int64, whence int) (int64, error) {
	position, err := r.Reader.Seek(offset, whence)
	if err == nil && position == 0 {
		r.readBytes = 0
	}
	return position, err
}

func TestAppStoreImageFormatBoundsPNGMetadata(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.White)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	// A valid RGB IHDR lets the format decoder stop before ancillary metadata.
	prefix := encoded.Bytes()[:33]
	for _, scenario := range []string{"huge declared chunk", "cumulative small chunks"} {
		t.Run(scenario, func(t *testing.T) {
			var content bytes.Buffer
			content.Write(prefix)
			if scenario == "huge declared chunk" {
				var header [8]byte
				binary.BigEndian.PutUint32(header[:4], 1<<30)
				copy(header[4:], "tEXt")
				content.Write(header[:])
			} else {
				chunk := make([]byte, 1024+12)
				binary.BigEndian.PutUint32(chunk[:4], 1024)
				copy(chunk[4:8], "tEXt")
				binary.BigEndian.PutUint32(chunk[len(chunk)-4:], crc32.ChecksumIEEE(chunk[4:len(chunk)-4]))
				for i := 0; i < 16*1024; i++ {
					content.Write(chunk)
				}
				content.Write(encoded.Bytes()[33:])
			}
			reader := &pngMetadataReadCounter{Reader: bytes.NewReader(content.Bytes())}
			format, err := ReadAppStoreImageFormatFrom(reader)
			if err == nil || !strings.Contains(err.Error(), "PNG metadata exceeds 16 MiB limit") {
				t.Fatalf("expected metadata limit error, got format %q error %v", format, err)
			}
			if reader.readBytes > 16<<20 {
				t.Fatalf("read %d metadata bytes, exceeding 16 MiB", reader.readBytes)
			}
			if scenario == "huge declared chunk" && reader.readBytes > 41 {
				t.Fatalf("oversized declaration consumed payload: read %d bytes", reader.readBytes)
			}
		})
	}
}
