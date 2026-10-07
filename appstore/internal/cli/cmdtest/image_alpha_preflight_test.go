package cmdtest

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImageUploadsRejectAlphaBeforeNetwork(t *testing.T) {
	for _, kind := range []string{"alpha", "truecolor-tRNS"} {
		for _, command := range []string{"screenshots", "creative"} {
			t.Run(kind+"/"+command, func(t *testing.T) {
				setupAuth(t)
				img := image.NewNRGBA(image.Rect(0, 0, 640, 960))
				if kind == "truecolor-tRNS" {
					for i := 0; i < len(img.Pix); i += 4 {
						img.Pix[i+3] = 255
					}
				}
				var data bytes.Buffer
				if err := png.Encode(&data, img); err != nil {
					t.Fatal(err)
				}
				content := data.Bytes()
				if kind == "truecolor-tRNS" {
					chunk := make([]byte, 18)
					binary.BigEndian.PutUint32(chunk, 6)
					copy(chunk[4:8], "tRNS")
					binary.BigEndian.PutUint32(chunk[14:], crc32.ChecksumIEEE(chunk[4:14]))
					content = append(append(append([]byte{}, content[:33]...), chunk...), content[33:]...)
				}
				path := filepath.Join(t.TempDir(), "alpha.png")
				if err := os.WriteFile(path, content, 0o600); err != nil {
					t.Fatal(err)
				}
				old := http.DefaultTransport
				t.Cleanup(func() { http.DefaultTransport = old })
				calls := 0
				http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
					calls++
					return statusJSONResponse(`{"data":[],"links":{}}`), nil
				})
				args := []string{"screenshots", "upload", "--version-localization", "LOC", "--path", path, "--device-type", "IPHONE_35"}
				if command == "creative" {
					args = []string{"asset-library", "images", "upload", "--library-id", "lib", "--file", path}
				}
				stdout, stderr, err := runRootCommand(t, args)
				if err == nil || !strings.Contains(err.Error(), "alpha channel or transparency") {
					t.Fatalf("expected alpha rejection, got %v; stdout=%s; stderr=%s", err, stdout, stderr)
				}
				if calls != 0 {
					t.Fatalf("made %d network calls before alpha rejection", calls)
				}
			})
		}
	}
}
