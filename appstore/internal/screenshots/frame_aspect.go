package screenshots

import (
	"fmt"
	"math"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

// frameAspectTolerance is the relative aspect-ratio difference between an
// input and the device screen that Koubou absorbs without visible bands.
// Simulator captures of sibling devices (for example an iPhone 17 Pro Max
// capture on the iPhone Air frame) differ by well under 1%.
const frameAspectTolerance = 0.02

// FrameAspectMismatch describes an input whose aspect ratio differs from the
// device screen, so Koubou letterboxes it inside the frame.
type FrameAspectMismatch struct {
	InputWidth   int
	InputHeight  int
	ScreenWidth  int
	ScreenHeight int
}

// CheckFrameInputAspect reads the PNG at path and returns a mismatch when its
// aspect ratio differs from device's screen by more than frameAspectTolerance.
// It returns nil when the aspect ratios match or the device has no fixed
// screen size. The Mac canvas profile is checked too: Koubou 0.20.0 fits a
// canvas screenshot inside the upload size and leaves background bands, as it
// does inside a bezel.
func CheckFrameInputAspect(path string, device FrameDevice) (*FrameAspectMismatch, error) {
	spec, ok := frameDeviceKoubouSpecs[device]
	if !ok {
		return nil, fmt.Errorf("no Koubou mapping configured for device %q", device)
	}
	screenWidth, screenHeight, ok := spec.outputDimensions()
	if !ok || screenWidth <= 0 || screenHeight <= 0 {
		return nil, nil
	}
	dimensions, err := asc.ReadImageDimensions(path)
	if err != nil {
		return nil, err
	}
	if dimensions.Width <= 0 || dimensions.Height <= 0 {
		return nil, nil
	}
	inputAspect := float64(dimensions.Width) / float64(dimensions.Height)
	screenAspect := float64(screenWidth) / float64(screenHeight)
	if math.Abs(inputAspect-screenAspect)/screenAspect <= frameAspectTolerance {
		return nil, nil
	}
	return &FrameAspectMismatch{
		InputWidth:   dimensions.Width,
		InputHeight:  dimensions.Height,
		ScreenWidth:  screenWidth,
		ScreenHeight: screenHeight,
	}, nil
}
