package assetlibrary

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLibraryVideoSpecRejectsIncompatibleCategoryAndAudio(t *testing.T) {
	reference := []byte(`{"data":[{"attributes":{"videoSpecs":[{"dimensions":{"minWidth":1920,"maxWidth":1920,"minHeight":886,"maxHeight":886},"compatiblePlacementTypes":["APP_PREVIEW"],"frameRates":[{"minFps":23,"maxFps":30}],"duration":{"min":"PT15S","max":"PT30S"},"audioRequired":true,"fileExtensions":[".mp4"],"mimeTypes":["video/mp4"],"maxFileSize":524288000}]}}]}`)
	p := libraryVideoProperties{Width: 1920, Height: 886, FPS: 29.97, Duration: 15, Audio: true, MIMEType: "video/mp4"}
	if err := matchLibraryVideoSpec(reference, "APP_SCREENSHOTS_AND_PREVIEWS", p, 100, ".mp4"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		category string
		p        libraryVideoProperties
	}{
		{"CREATIVE_ASSETS", p},
		{"APP_SCREENSHOTS_AND_PREVIEWS", libraryVideoProperties{Width: 1920, Height: 886, FPS: 30, Duration: 15}},
		{"APP_SCREENSHOTS_AND_PREVIEWS", libraryVideoProperties{Width: 1920, Height: 886, FPS: 60, Duration: 15, Audio: true, MIMEType: "video/mp4"}},
		{"APP_SCREENSHOTS_AND_PREVIEWS", libraryVideoProperties{Width: 1920, Height: 886, FPS: 30, Duration: 5, Audio: true, MIMEType: "video/mp4"}},
	} {
		tc.p.MIMEType = "video/mp4"
		if err := matchLibraryVideoSpec(reference, tc.category, tc.p, 100, ".mp4"); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
}

func TestLibraryVideoInspectionDeadlineClosesInheritedPipe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX descendant fixture")
	}
	dir := t.TempDir()
	probe := `#!/bin/sh
sleep 3 &
sleep 3
`
	if err := os.WriteFile(filepath.Join(dir, "ffprobe"), []byte(probe), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := inspectLibraryVideo(ctx, "unused.mp4")
	if err == nil || !strings.Contains(err.Error(), "deadline") || time.Since(started) > 2*time.Second {
		t.Fatalf("inspection deadline not bounded: elapsed=%v err=%v", time.Since(started), err)
	}
}

func TestLibraryVideoInspectionPreservesMOVMP4M4V(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX ffprobe fixture")
	}
	dir := t.TempDir()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for extension, fixture := range map[string]struct{ brand, mime string }{".mov": {"qt  ", "video/quicktime"}, ".mp4": {"isom", "video/mp4"}, ".m4v": {"M4V ", "video/x-m4v"}} {
		probe := `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"video","codec_name":"h264","width":1920,"height":1280,"avg_frame_rate":"30/1"}],"format":{"duration":"5","format_name":"mov,mp4,m4a,3gp,3g2,mj2","tags":{"major_brand":"BRAND"}}}'
`
		probe = strings.ReplaceAll(probe, "BRAND", fixture.brand)
		if err := os.WriteFile(filepath.Join(dir, "ffprobe"), []byte(probe), 0o700); err != nil {
			t.Fatal(err)
		}
		p, err := inspectLibraryVideo(context.Background(), "video"+extension)
		if err != nil || p.MIMEType != fixture.mime || p.Codec != "h264" {
			t.Fatalf("%s properties=%+v err=%v", extension, p, err)
		}
	}
}

// The real-container fixtures in testdata are one-frame 176x144 clips made with
// ffmpeg -f lavfi -i color=c=black:s=176x144:r=30 -frames:v 1 -an, using
// -c:v mpeg4 -f 3gp for 3gp-renamed.mp4 and -c:v libx264 with the mp4, mov and
// ipod muxers for video.mp4, video.mov and video.m4v.
func TestLibraryVideoRejectsReal3GPRenamedMP4(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe required for actual-container fixture")
	}
	if p, err := inspectLibraryVideo(context.Background(), filepath.Join("testdata", "3gp-renamed.mp4")); err == nil || !strings.Contains(err.Error(), "container") {
		t.Fatalf("renamed real 3GP accepted: %+v err=%v", p, err)
	}
}

func TestLibraryVideoContainerCompatibleBrand(t *testing.T) {
	mime, err := libraryVideoContainerMIME("dash", "iso6mp41")
	if err != nil || mime != "video/mp4" {
		t.Fatalf("compatible MPEG4 family rejected: %s %v", mime, err)
	}
	for _, major := range []string{"3gp4", "3g2a", "mjp2", "unknown"} {
		if _, err := libraryVideoContainerMIME(major, ""); err == nil {
			t.Fatalf("unsupported brand %s accepted", major)
		}
	}
}

func TestLibraryVideoRealSupportedContainers(t *testing.T) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe required for actual-container fixtures")
	}
	for _, fixture := range []struct{ extension, mime string }{{".mp4", "video/mp4"}, {".mov", "video/quicktime"}, {".m4v", "video/x-m4v"}} {
		t.Run(fixture.extension, func(t *testing.T) {
			p, err := inspectLibraryVideo(context.Background(), filepath.Join("testdata", "video"+fixture.extension))
			if err != nil || p.MIMEType != fixture.mime || p.Codec != "h264" {
				t.Fatalf("actual %s properties=%+v err=%v", fixture.extension, p, err)
			}
		})
	}
}

func TestLibraryVideoAspectRatioForms(t *testing.T) {
	for _, raw := range []string{`"21:9"`, `{"width":21,"height":9}`, `{"width":2.1,"height":0.9}`} {
		ratio, err := normalizeLibraryVideoAspectRatio(json.RawMessage(raw))
		if err != nil || ratio.RatString() != "7/3" {
			t.Fatalf("%s ratio=%v err=%v", raw, ratio, err)
		}
	}
	for _, raw := range []string{`"0:9"`, `"-21:9"`, `"NaN:9"`, `"Inf:9"`, `"1e309:9"`, `{"width":0,"height":9}`, `{"width":21,"height":-9}`, `{"width":21}`, `null`} {
		if ratio, err := normalizeLibraryVideoAspectRatio(json.RawMessage(raw)); err == nil {
			t.Fatalf("invalid %s accepted: %v", raw, ratio)
		}
	}
}
