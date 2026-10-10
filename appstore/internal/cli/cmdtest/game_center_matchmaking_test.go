package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"testing"
)

func TestGameCenterMatchmakingQueuesCreateValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing reference-name",
			args: []string{"game-center", "matchmaking", "queues", "create", "--rule-set-id", "RULE_SET_ID"},
		},
		{
			name: "missing rule-set-id",
			args: []string{"game-center", "matchmaking", "queues", "create", "--reference-name", "Queue"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}

func TestGameCenterMatchmakingQueuesUpdateValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing id",
			args: []string{"game-center", "matchmaking", "queues", "update"},
		},
		{
			name: "missing update flags",
			args: []string{"game-center", "matchmaking", "queues", "update", "--id", "QUEUE_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}

func TestGameCenterMatchmakingQueuesDeleteValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing id",
			args: []string{"game-center", "matchmaking", "queues", "delete", "--confirm"},
		},
		{
			name: "missing confirm",
			args: []string{"game-center", "matchmaking", "queues", "delete", "--id", "QUEUE_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}

func TestGameCenterMatchmakingRuleSetsCreateValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing reference-name",
			args: []string{"game-center", "matchmaking", "rule-sets", "create", "--rule-language-version", "1", "--min-players", "2", "--max-players", "8"},
		},
		{
			name: "missing rule-language-version",
			args: []string{"game-center", "matchmaking", "rule-sets", "create", "--reference-name", "Rules", "--min-players", "2", "--max-players", "8"},
		},
		{
			name: "missing min/max",
			args: []string{"game-center", "matchmaking", "rule-sets", "create", "--reference-name", "Rules", "--rule-language-version", "1"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}

func TestGameCenterMatchmakingRuleSetsUpdateValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing id",
			args: []string{"game-center", "matchmaking", "rule-sets", "update"},
		},
		{
			name: "missing update flags",
			args: []string{"game-center", "matchmaking", "rule-sets", "update", "--id", "RULE_SET_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}

func TestGameCenterMatchmakingRuleSetsDeleteValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing id",
			args: []string{"game-center", "matchmaking", "rule-sets", "delete", "--confirm"},
		},
		{
			name: "missing confirm",
			args: []string{"game-center", "matchmaking", "rule-sets", "delete", "--id", "RULE_SET_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}

func TestGameCenterMatchmakingRulesListValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "matchmaking", "rules", "list"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestGameCenterMatchmakingRulesCreateValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing rule-set-id",
			args: []string{"game-center", "matchmaking", "rules", "create", "--reference-name", "Rule", "--description", "Desc", "--type", "MATCH", "--expression", "true"},
		},
		{
			name: "missing reference-name",
			args: []string{"game-center", "matchmaking", "rules", "create", "--rule-set-id", "RULE_SET_ID", "--description", "Desc", "--type", "MATCH", "--expression", "true"},
		},
		{
			name: "missing description",
			args: []string{"game-center", "matchmaking", "rules", "create", "--rule-set-id", "RULE_SET_ID", "--reference-name", "Rule", "--type", "MATCH", "--expression", "true"},
		},
		{
			name: "missing type",
			args: []string{"game-center", "matchmaking", "rules", "create", "--rule-set-id", "RULE_SET_ID", "--reference-name", "Rule", "--description", "Desc", "--expression", "true"},
		},
		{
			name: "missing expression",
			args: []string{"game-center", "matchmaking", "rules", "create", "--rule-set-id", "RULE_SET_ID", "--reference-name", "Rule", "--description", "Desc", "--type", "MATCH"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}

func TestGameCenterMatchmakingRulesUpdateValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing id",
			args: []string{"game-center", "matchmaking", "rules", "update"},
		},
		{
			name: "missing update flags",
			args: []string{"game-center", "matchmaking", "rules", "update", "--id", "RULE_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}

func TestGameCenterMatchmakingRulesDeleteValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing id",
			args: []string{"game-center", "matchmaking", "rules", "delete", "--confirm"},
		},
		{
			name: "missing confirm",
			args: []string{"game-center", "matchmaking", "rules", "delete", "--id", "RULE_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}

func TestGameCenterMatchmakingTeamsListValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "matchmaking", "teams", "list"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestGameCenterMatchmakingTeamsCreateValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing rule-set-id",
			args: []string{"game-center", "matchmaking", "teams", "create", "--reference-name", "Team", "--min-players", "1", "--max-players", "4"},
		},
		{
			name: "missing reference-name",
			args: []string{"game-center", "matchmaking", "teams", "create", "--rule-set-id", "RULE_SET_ID", "--min-players", "1", "--max-players", "4"},
		},
		{
			name: "missing min/max",
			args: []string{"game-center", "matchmaking", "teams", "create", "--rule-set-id", "RULE_SET_ID", "--reference-name", "Team"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}

func TestGameCenterMatchmakingTeamsUpdateValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing id",
			args: []string{"game-center", "matchmaking", "teams", "update"},
		},
		{
			name: "missing update flags",
			args: []string{"game-center", "matchmaking", "teams", "update", "--id", "TEAM_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}

func TestGameCenterMatchmakingTeamsDeleteValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing id",
			args: []string{"game-center", "matchmaking", "teams", "delete", "--confirm"},
		},
		{
			name: "missing confirm",
			args: []string{"game-center", "matchmaking", "teams", "delete", "--id", "TEAM_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}

func TestGameCenterMatchmakingMetricsQueueValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "queue sizes missing queue-id",
			args: []string{"game-center", "matchmaking", "metrics", "queue-sizes", "--granularity", "P1D"},
		},
		{
			name: "queue requests missing queue-id",
			args: []string{"game-center", "matchmaking", "metrics", "queue-requests", "--granularity", "P1D"},
		},
		{
			name: "queue sessions missing queue-id",
			args: []string{"game-center", "matchmaking", "metrics", "queue-sessions", "--granularity", "P1D"},
		},
		{
			name: "experiment queue sizes missing queue-id",
			args: []string{"game-center", "matchmaking", "metrics", "experiment-queue-sizes", "--granularity", "P1D"},
		},
		{
			name: "experiment queue requests missing queue-id",
			args: []string{"game-center", "matchmaking", "metrics", "experiment-queue-requests", "--granularity", "P1D"},
		},
		{
			name: "queue sizes missing granularity",
			args: []string{"game-center", "matchmaking", "metrics", "queue-sizes", "--queue-id", "QUEUE_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}

func TestGameCenterMatchmakingMetricsRuleValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "boolean rule results missing rule-id",
			args: []string{"game-center", "matchmaking", "metrics", "rule-boolean-results", "--granularity", "P1D"},
		},
		{
			name: "number rule results missing rule-id",
			args: []string{"game-center", "matchmaking", "metrics", "rule-number-results", "--granularity", "P1D"},
		},
		{
			name: "rule errors missing rule-id",
			args: []string{"game-center", "matchmaking", "metrics", "rule-errors", "--granularity", "P1D"},
		},
		{
			name: "boolean rule results missing granularity",
			args: []string{"game-center", "matchmaking", "metrics", "rule-boolean-results", "--rule-id", "RULE_ID"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, _ := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected ErrHelp, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}

func TestGameCenterMatchmakingRuleSetTestsCreateValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "matchmaking", "rule-set-tests", "create"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestGameCenterMatchmakingRuleSetQueuesListValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "matchmaking", "rule-sets", "queues", "list"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}
