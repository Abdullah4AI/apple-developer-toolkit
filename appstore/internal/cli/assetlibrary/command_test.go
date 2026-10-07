package assetlibrary

import (
	"context"
	"errors"
	"flag"
	"testing"
)

func TestCommandGroupHelp(t *testing.T) {
	cmd := Command()
	if len(cmd.Subcommands) == 0 || cmd.UsageFunc == nil || !errors.Is(cmd.Exec(context.Background(), nil), flag.ErrHelp) {
		t.Fatal("expected read command group with standard help")
	}
	for _, child := range cmd.Subcommands {
		if child.Name == "upload" || child.Name == "submit" || child.Name == "delete" {
			t.Fatal("unverified mutation exposed")
		}
	}
}
