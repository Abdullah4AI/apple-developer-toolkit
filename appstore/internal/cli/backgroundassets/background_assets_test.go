package backgroundassets

import (
	"context"
	"errors"
	"flag"
	"path/filepath"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/cli/shared"
)

func TestBackgroundAssetsListCommand_MissingApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	cmd := BackgroundAssetsListCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --app is missing, got %v", err)
	}
}

func TestBackgroundAssetsGetCommand_MissingID(t *testing.T) {
	cmd := BackgroundAssetsGetCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestBackgroundAssetsCreateCommand_MissingApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	cmd := BackgroundAssetsCreateCommand()
	if err := cmd.FlagSet.Parse([]string{"--asset-pack-identifier", "com.example.assetpack"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --app is missing, got %v", err)
	}
}

func TestBackgroundAssetsCreateCommand_MissingAssetPackIdentifier(t *testing.T) {
	cmd := BackgroundAssetsCreateCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "APP_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --asset-pack-identifier is missing, got %v", err)
	}
}

func TestBackgroundAssetsUpdateCommand_MissingID(t *testing.T) {
	cmd := BackgroundAssetsUpdateCommand()
	if err := cmd.FlagSet.Parse([]string{"--archived", "true"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --id is missing, got %v", err)
	}
}

func TestBackgroundAssetsUpdateCommand_MissingArchived(t *testing.T) {
	cmd := BackgroundAssetsUpdateCommand()
	if err := cmd.FlagSet.Parse([]string{"--id", "ASSET_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --archived is missing, got %v", err)
	}
}

func TestBackgroundAssetsVersionsListCommand_MissingAssetID(t *testing.T) {
	cmd := BackgroundAssetsVersionsListCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --background-asset-id is missing, got %v", err)
	}
}

func TestBackgroundAssetsVersionsGetCommand_MissingID(t *testing.T) {
	cmd := BackgroundAssetsVersionsGetCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --version-id is missing, got %v", err)
	}
}

func TestBackgroundAssetsVersionsCreateCommand_MissingAssetID(t *testing.T) {
	cmd := BackgroundAssetsVersionsCreateCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --background-asset-id is missing, got %v", err)
	}
}

func TestBackgroundAssetsUploadFilesListCommand_MissingVersionID(t *testing.T) {
	cmd := BackgroundAssetsUploadFilesListCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --version-id is missing, got %v", err)
	}
}

func TestBackgroundAssetsUploadFilesGetCommand_MissingID(t *testing.T) {
	cmd := BackgroundAssetsUploadFilesGetCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --upload-file-id is missing, got %v", err)
	}
}

func TestBackgroundAssetsUploadFilesCreateCommand_MissingVersionID(t *testing.T) {
	cmd := BackgroundAssetsUploadFilesCreateCommand()
	if err := cmd.FlagSet.Parse([]string{"--file", "./asset.zip", "--asset-type", "ASSET"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --version-id is missing, got %v", err)
	}
}

func TestBackgroundAssetsUploadFilesCreateCommand_MissingFile(t *testing.T) {
	cmd := BackgroundAssetsUploadFilesCreateCommand()
	if err := cmd.FlagSet.Parse([]string{"--version-id", "VERSION_ID", "--asset-type", "ASSET"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --file is missing, got %v", err)
	}
}

func TestBackgroundAssetsUploadFilesCreateCommand_MissingAssetType(t *testing.T) {
	cmd := BackgroundAssetsUploadFilesCreateCommand()
	if err := cmd.FlagSet.Parse([]string{"--version-id", "VERSION_ID", "--file", "./asset.zip"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --asset-type is missing, got %v", err)
	}
}

func TestBackgroundAssetsUploadFilesUpdateCommand_MissingID(t *testing.T) {
	cmd := BackgroundAssetsUploadFilesUpdateCommand()
	if err := cmd.FlagSet.Parse([]string{"--uploaded", "true"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --upload-file-id is missing, got %v", err)
	}
}

func TestBackgroundAssetsUploadFilesUpdateCommand_MissingUploaded(t *testing.T) {
	cmd := BackgroundAssetsUploadFilesUpdateCommand()
	if err := cmd.FlagSet.Parse([]string{"--upload-file-id", "UPLOAD_ID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --uploaded is missing, got %v", err)
	}
}

func TestBackgroundAssetsUploadFilesUpdateCommand_FileRequiresChecksumBeforeFileOrClientAccess(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "missing.zip")
	clientFactoryCalled := false
	restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		clientFactoryCalled = true
		return nil, errors.New("client factory must not run during flag validation")
	})
	defer restore()

	cmd := BackgroundAssetsUploadFilesUpdateCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--upload-file-id", "UPLOAD_ID",
		"--uploaded", "true",
		"--file", filePath,
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp when --file is used without --checksum, got %v", err)
	}
	if got, want := err.Error(), "--file requires --checksum"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
	if clientFactoryCalled {
		t.Fatal("client factory ran before --file/--checksum validation")
	}
}
