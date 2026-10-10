package actors

import (
	"context"
	"errors"
	"flag"
	"testing"
)

func TestActorsValidationErrors(t *testing.T) {
	t.Run("list missing id", func(t *testing.T) {
		cmd := ActorsListCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("get missing id", func(t *testing.T) {
		cmd := ActorsGetCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})
}

func TestNormalizeActorFields(t *testing.T) {
	got, err := normalizeActorFields("actorType,userEmail")
	if err != nil {
		t.Fatalf("expected valid fields, got err=%v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(got))
	}

	if _, err := normalizeActorFields("badField"); err == nil {
		t.Fatal("expected error for invalid field")
	}
}
