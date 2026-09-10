package apps

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/asc"
)

func TestAppsCommandRegistersPublished(t *testing.T) {
	cmd := AppsCommand()
	for _, subcommand := range cmd.Subcommands {
		if subcommand.Name == "published" {
			if !strings.Contains(cmd.LongHelp, "asc apps published") {
				t.Fatalf("apps help does not mention published: %q", cmd.LongHelp)
			}
			return
		}
	}
	t.Fatal("expected apps published subcommand")
}

func TestAppsPublishedCommandDefaultsToJSON(t *testing.T) {
	cmd := AppsPublishedCommand()

	output := cmd.FlagSet.Lookup("output")
	if output == nil {
		t.Fatal("expected --output flag")
	}
	if output.DefValue != "json" {
		t.Fatalf("default output = %q, want json", output.DefValue)
	}
}

func TestAppsPublishedCommandRejectsPositionalArgumentsBeforeClientCreation(t *testing.T) {
	originalFactory := appsPublishedClientFactory
	t.Cleanup(func() { appsPublishedClientFactory = originalFactory })

	clientFactoryCalls := 0
	appsPublishedClientFactory = func() (*asc.Client, error) {
		clientFactoryCalls++
		return nil, errors.New("client factory must not be called")
	}

	err := AppsPublishedCommand().Exec(context.Background(), []string{"unexpected"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
	if err.Error() != "apps published does not accept positional arguments" {
		t.Fatalf("unexpected error: %v", err)
	}
	if clientFactoryCalls != 0 {
		t.Fatalf("client factory calls = %d, want 0", clientFactoryCalls)
	}
}
