package screenshots

import (
	"fmt"
	"math"
	"strings"
)

// TextPosition places title and subtitle overlays above or below the device.
type TextPosition string

const (
	TextPositionTop    TextPosition = "top"
	TextPositionBottom TextPosition = "bottom"
)

// TextPositionValues returns the allowed --text-position values.
func TextPositionValues() []string {
	return []string{string(TextPositionTop), string(TextPositionBottom)}
}

// ParseTextPosition normalizes a --text-position value. Empty selects top.
func ParseTextPosition(raw string) (TextPosition, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(TextPositionTop):
		return TextPositionTop, nil
	case string(TextPositionBottom):
		return TextPositionBottom, nil
	default:
		return "", fmt.Errorf("unsupported text position %q (allowed: %s)", raw, strings.Join(TextPositionValues(), ", "))
	}
}

// koubouCenterAlignment anchors every generated text and image item on its
// horizontal center. Since Koubou 0.19.0, content alignment sets the true
// horizontal anchor for both text and images, so emitting it explicitly keeps
// the 50% x positions below centered regardless of Koubou's defaults.
const koubouCenterAlignment = "center"

// Canvas (mac) layout with text below the window.
const (
	canvasBottomTitleY        = "84%"
	canvasBottomSubtitleY     = "88%"
	canvasBottomSubtitleSoloY = "84%"
	canvasWindowBottomTextY   = "40%" // window pushed up to make room for text overlays
)

// Bezel layout with text overlays. The text band takes the top or bottom 22%
// of the canvas and the framed device is scaled to fit the remaining space. Font sizes follow the canvas short edge so landscape (TV),
// tablet, phone, and watch canvases keep proportional text.
const (
	bezelDeviceMaxWidthFrac  = 0.90
	bezelDeviceMaxHeightFrac = 0.74
	bezelTitleSizeFrac       = 0.07
	bezelSubtitleSizeFrac    = 0.04
	bezelTextMaxWidthFrac    = 0.90

	bezelTopTitleY           = "8%"
	bezelTopSubtitleY        = "15%"
	bezelTopSubtitleSoloY    = "11%"
	bezelTopDeviceY          = "61%" // center of the space below a 22% text band
	bezelBottomTitleY        = "85%"
	bezelBottomSubtitleY     = "92%"
	bezelBottomSubtitleSoloY = "89%"
	bezelBottomDeviceY       = "39%" // center of the space above a 22% text band
)

type textLayout struct {
	titleY        string
	subtitleY     string
	subtitleSoloY string
	titleSize     int
	subtitleSize  int
	maxWidth      int
}

func canvasTextLayout(position TextPosition) textLayout {
	layout := textLayout{
		titleY:        canvasTitleY,
		subtitleY:     canvasSubtitleY,
		subtitleSoloY: canvasSubtitleSoloY,
		titleSize:     canvasTitleFontSize,
		subtitleSize:  canvasSubtitleFontSize,
	}
	if position == TextPositionBottom {
		layout.titleY = canvasBottomTitleY
		layout.subtitleY = canvasBottomSubtitleY
		layout.subtitleSoloY = canvasBottomSubtitleSoloY
	}
	return layout
}

// textContentItems renders the title and subtitle overlays for layout.
func textContentItems(opts *CanvasOptions, layout textLayout) []koubouDefaultContentItem {
	if opts == nil {
		return nil
	}
	items := make([]koubouDefaultContentItem, 0, 2)
	fontFamily := opts.Font
	if opts.FontFile != nil {
		fontFamily = opts.FontFile.workName()
	}
	if opts.Title != "" {
		color := opts.TitleColor
		if color == "" {
			color = canvasDefaultTitleColor
		}
		items = append(items, koubouDefaultContentItem{
			Type:       "text",
			Content:    opts.Title,
			Position:   [2]string{"50%", layout.titleY},
			Size:       layout.titleSize,
			Weight:     "bold",
			Color:      color,
			FontFamily: fontFamily,
			Alignment:  koubouCenterAlignment,
			MaxWidth:   layout.maxWidth,
			Box:        opts.TextBox.koubouBox(layout.titleSize),
		})
	}
	if opts.Subtitle != "" {
		color := opts.SubtitleColor
		if color == "" {
			color = canvasDefaultSubtitleColor
		}
		subtitleY := layout.subtitleY
		if opts.Title == "" {
			subtitleY = layout.subtitleSoloY
		}
		items = append(items, koubouDefaultContentItem{
			Type:       "text",
			Content:    opts.Subtitle,
			Position:   [2]string{"50%", subtitleY},
			Size:       layout.subtitleSize,
			Color:      color,
			FontFamily: fontFamily,
			Alignment:  koubouCenterAlignment,
			MaxWidth:   layout.maxWidth,
			Box:        opts.TextBox.koubouBox(layout.subtitleSize),
		})
	}
	return items
}

// bezelContentItems places a framed screenshot on the output canvas. Without
// text the frame keeps Koubou's full-canvas fit. With text, the frame is
// scaled from its native pinned size into the space beside the text band.
func bezelContentItems(absInputPath string, spec frameDeviceKoubouSpec, opts *CanvasOptions) []koubouDefaultContentItem {
	image := koubouDefaultContentItem{
		Type:      "image",
		Asset:     absInputPath,
		Position:  [2]string{"50%", "50%"},
		Scale:     1,
		Frame:     boolPtr(true),
		Alignment: koubouCenterAlignment,
	}
	canvasWidth, canvasHeight, ok := spec.outputDimensions()
	if opts == nil || !opts.hasText() || !ok || spec.FrameWidth <= 0 || spec.FrameHeight <= 0 {
		return append(textContentItems(opts, canvasTextLayout(TextPositionTop)), image)
	}

	shortEdge := float64(min(canvasWidth, canvasHeight))
	layout := textLayout{
		titleY:        bezelTopTitleY,
		subtitleY:     bezelTopSubtitleY,
		subtitleSoloY: bezelTopSubtitleSoloY,
		titleSize:     int(math.Round(shortEdge * bezelTitleSizeFrac)),
		subtitleSize:  int(math.Round(shortEdge * bezelSubtitleSizeFrac)),
		maxWidth:      int(math.Round(float64(canvasWidth) * bezelTextMaxWidthFrac)),
	}
	image.Position[1] = bezelTopDeviceY
	if opts.TextPosition == TextPositionBottom {
		layout.titleY = bezelBottomTitleY
		layout.subtitleY = bezelBottomSubtitleY
		layout.subtitleSoloY = bezelBottomSubtitleSoloY
		image.Position[1] = bezelBottomDeviceY
	}
	scale := math.Min(
		float64(canvasWidth)*bezelDeviceMaxWidthFrac/float64(spec.FrameWidth),
		float64(canvasHeight)*bezelDeviceMaxHeightFrac/float64(spec.FrameHeight),
	)
	image.Scale = math.Round(scale*10000) / 10000
	return append(textContentItems(opts, layout), image)
}
