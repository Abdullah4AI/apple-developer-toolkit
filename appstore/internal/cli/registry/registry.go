package registry

import (
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/accessibility"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/account"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/actors"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/agerating"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/agreements"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/alternativedistribution"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/analytics"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/androidiosmapping"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/app_events"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/appclips"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/apps"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/auth"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/backgroundassets"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/betaapplocalizations"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/betabuildlocalizations"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/buildbundles"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/buildlocalizations"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/builds"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/bundleids"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/categories"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/certificates"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/completion"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/crashes"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/devices"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/diffcmd"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/docs"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/encryption"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/eula"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/feedback"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/finance"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/gamecenter"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/iap"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/initcmd"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/insights"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/install"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/localizations"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/marketplace"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/merchantids"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/metadata"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/migrate"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/nominations"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/notarization"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/notify"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/offercodes"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/passtypeids"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/performance"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/preorders"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/prerelease"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/pricing"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/productpages"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/profiles"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/promotedpurchases"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/publish"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/releasenotes"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/reviews"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/routingcoverage"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/sandbox"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/screenshots"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/signing"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/status"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/submit"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/subscriptions"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/testflight"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/users"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/validate"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/versions"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/videopreviews"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/web"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/webhooks"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/winbackoffers"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/workflow"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/xcodecloud"
)

// VersionCommand returns a version subcommand.
func VersionCommand(version string) *ffcli.Command {
	return &ffcli.Command{
		Name:       "version",
		ShortUsage: "asc version",
		ShortHelp:  "Print version information and exit.",
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			fmt.Println(version)
			return nil
		},
	}
}

// Subcommands returns all root subcommands in display order.
func Subcommands(version string) []*ffcli.Command {
	subs := []*ffcli.Command{
		auth.AuthCommand(),
		auth.AuthDoctorCommand(),
		web.WebCommand(),
		account.AccountCommand(),
		install.InstallSkillsCommand(),
		initcmd.InitCommand(),
		docs.DocsCommand(),
		diffcmd.DiffCommand(),
		status.StatusCommand(),
		insights.InsightsCommand(),
		releasenotes.ReleaseNotesCommand(),
		feedback.FeedbackCommand(),
		crashes.CrashesCommand(),
		reviews.ReviewsCommand(),
		reviews.ReviewCommand(),
		analytics.AnalyticsCommand(),
		performance.PerformanceCommand(),
		finance.FinanceCommand(),
		apps.AppsCommand(),
		appclips.AppClipsCommand(),
		androidiosmapping.AndroidIosMappingCommand(),
		apps.AppSetupCommand(),
		apps.AppTagsCommand(),
		marketplace.MarketplaceCommand(),
		alternativedistribution.Command(),
		webhooks.WebhooksCommand(),
		nominations.NominationsCommand(),
		bundleids.BundleIDsCommand(),
		merchantids.MerchantIDsCommand(),
		certificates.CertificatesCommand(),
		passtypeids.PassTypeIDsCommand(),
		profiles.ProfilesCommand(),
		offercodes.OfferCodesCommand(),
		winbackoffers.WinBackOffersCommand(),
		users.UsersCommand(),
		actors.ActorsCommand(),
		devices.DevicesCommand(),
		testflight.TestFlightCommand(),
		builds.BuildsCommand(),
		buildbundles.BuildBundlesCommand(),
		publish.PublishCommand(),
		workflow.WorkflowCommand(),
		versions.VersionsCommand(),
		productpages.ProductPagesCommand(),
		routingcoverage.RoutingCoverageCommand(),
		apps.AppInfoCommand(),
		apps.AppInfosCommand(),
		eula.EULACommand(),
		agreements.AgreementsCommand(),
		pricing.PricingCommand(),
		preorders.PreOrdersCommand(),
		prerelease.PreReleaseVersionsCommand(),
		localizations.LocalizationsCommand(),
		metadata.MetadataCommand(),
		screenshots.ScreenshotsCommand(),
		videopreviews.VideoPreviewsCommand(),
		backgroundassets.BackgroundAssetsCommand(),
		buildlocalizations.BuildLocalizationsCommand(),
		betaapplocalizations.BetaAppLocalizationsCommand(),
		betabuildlocalizations.BetaBuildLocalizationsCommand(),
		sandbox.SandboxCommand(),
		signing.SigningCommand(),
		notarization.NotarizationCommand(),
		iap.IAPCommand(),
		app_events.Command(),
		subscriptions.SubscriptionsCommand(),
		submit.SubmitCommand(),
		validate.ValidateCommand(),
		xcodecloud.XcodeCloudCommand(),
		categories.CategoriesCommand(),
		agerating.AgeRatingCommand(),
		accessibility.AccessibilityCommand(),
		encryption.EncryptionCommand(),
		promotedpurchases.PromotedPurchasesCommand(),
		migrate.MigrateCommand(),
		notify.NotifyCommand(),
		gamecenter.GameCenterCommand(),
		VersionCommand(version),
	}

	subs = append(subs, completion.CompletionCommand(subs))
	return subs
}
