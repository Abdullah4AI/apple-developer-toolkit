package shots

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/screenshots"
)

type frameDevicesOutput struct {
	Default string                          `json:"default"`
	Devices []screenshots.FrameDeviceOption `json:"devices"`
}

// ShotsFramesListDevicesCommand returns the screenshots list-frame-devices subcommand.
func ShotsFramesListDevicesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list-frame-devices", flag.ExitOnError)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list-frame-devices",
		ShortUsage: "asc screenshots list-frame-devices [--output json]",
		ShortHelp:  "List supported frame devices and the default (experimental).",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			result := frameDevicesOutput{
				Default: string(screenshots.DefaultFrameDevice()),
				Devices: screenshots.FrameDeviceOptions(),
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
