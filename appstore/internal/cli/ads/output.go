package ads

import (
	"flag"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

// bindAdsRawOutputFlags keeps opaque Apple Ads envelopes lossless. The 99
// Platform operations return heterogeneous nested shapes, so pretending they
// have a generic table renderer would hide data.
func bindAdsRawOutputFlags(fs *flag.FlagSet) shared.OutputFlags {
	return shared.BindOutputFlagsWithAllowed(fs, "output", "json", "Output format: json", "json")
}

func validateAdsRawOutput(flags shared.OutputFlags) (string, error) {
	return shared.ValidateOutputFormatAllowed(*flags.Output, *flags.Pretty, "json")
}
