package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSubscriptionsIntroductoryOffersImport_MissingRequiredFlagsReturnUsage(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing subscription id",
			args:    []string{"subscriptions", "offers", "introductory", "import", "--input", "offers.csv"},
			wantErr: "Error: --subscription-id is required",
		},
		{
			name:    "missing input",
			args:    []string{"subscriptions", "offers", "introductory", "import", "--subscription-id", "SUB_ID"},
			wantErr: "Error: --input is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q in stderr, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestSubscriptionsIntroductoryOffersImport_InvalidDefaultDatesReturnUsage(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "invalid start date",
			args: []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "SUB_ID",
				"--input", "offers.csv",
				"--start-date", "2026-99-99",
			},
			wantErr: "--start-date must be in YYYY-MM-DD format",
		},
		{
			name: "invalid end date",
			args: []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "SUB_ID",
				"--input", "offers.csv",
				"--end-date", "2026-13-01",
			},
			wantErr: "--end-date must be in YYYY-MM-DD format",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q in stderr, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestSubscriptionsIntroductoryOffersImport_InvalidDefaultOfferFlagsReturnUsage(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "invalid offer duration",
			args: []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "SUB_ID",
				"--input", "offers.csv",
				"--offer-duration", "bad",
			},
			wantErr: "--offer-duration must be one of:",
		},
		{
			name: "invalid offer mode",
			args: []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "SUB_ID",
				"--input", "offers.csv",
				"--offer-mode", "bad",
			},
			wantErr: "--offer-mode must be one of:",
		},
		{
			name: "negative number of periods",
			args: []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "SUB_ID",
				"--input", "offers.csv",
				"--number-of-periods", "-1",
			},
			wantErr: "--number-of-periods must be greater than or equal to 0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q in stderr, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestSubscriptionsIntroductoryOffersImport_UnknownCSVColumnReturnsUsage(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	csvPath := filepath.Join(t.TempDir(), "offers.csv")
	if err := os.WriteFile(csvPath, []byte("territory,unknown\nUSA,value\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "import",
			"--subscription-id", "SUB_ID",
			"--input", csvPath,
		}); err != nil {
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
	if !strings.Contains(stderr, "unknown CSV column") {
		t.Fatalf("expected unknown column error, got %q", stderr)
	}
}

func TestSubscriptionsIntroductoryOffersImport_UnknownThreeLetterTerritoryReturnsUsage(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	csvPath := filepath.Join(t.TempDir(), "offers.csv")
	if err := os.WriteFile(csvPath, []byte("territory\nZZZ\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "import",
			"--subscription-id", "SUB_ID",
			"--input", csvPath,
			"--offer-duration", "ONE_WEEK",
			"--offer-mode", "FREE_TRIAL",
			"--number-of-periods", "1",
			"--dry-run",
		}); err != nil {
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
	if !strings.Contains(stderr, `row 1: territory "ZZZ" could not be mapped`) {
		t.Fatalf("expected territory mapping error, got %q", stderr)
	}
}

func TestSubscriptionsIntroductoryOffersImport_InvalidCSVSchemaReturnsUsage(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "empty file",
			body:    "",
			wantErr: "CSV file is empty",
		},
		{
			name:    "missing territory header",
			body:    "offer_mode\nFREE_TRIAL\n",
			wantErr: `CSV header must include required column "territory"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			csvPath := filepath.Join(t.TempDir(), "offers.csv")
			if err := os.WriteFile(csvPath, []byte(test.body), 0o600); err != nil {
				t.Fatalf("WriteFile() error: %v", err)
			}

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"subscriptions", "offers", "introductory", "import",
					"--subscription-id", "SUB_ID",
					"--input", csvPath,
				}); err != nil {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q in stderr, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestSubscriptionsIntroductoryOffersImport_InvalidRowDatesReturnUsage(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "invalid row start date",
			body:    "territory,start_date\nUSA,2026-15-01\n",
			wantErr: "row 1: --start-date must be in YYYY-MM-DD format",
		},
		{
			name:    "invalid row end date",
			body:    "territory,end_date\nUSA,2026-04-99\n",
			wantErr: "row 1: --end-date must be in YYYY-MM-DD format",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			csvPath := filepath.Join(t.TempDir(), "offers.csv")
			if err := os.WriteFile(csvPath, []byte(test.body), 0o600); err != nil {
				t.Fatalf("WriteFile() error: %v", err)
			}

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"subscriptions", "offers", "introductory", "import",
					"--subscription-id", "SUB_ID",
					"--input", csvPath,
				}); err != nil {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q in stderr, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestSubscriptionsIntroductoryOffersImport_InvalidRowOfferEnumsReturnUsage(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "invalid row offer mode",
			body:    "territory,offer_mode\nUSA,bad\n",
			wantErr: "row 1: --offer-mode must be one of:",
		},
		{
			name:    "invalid row offer duration",
			body:    "territory,offer_duration\nUSA,bad\n",
			wantErr: "row 1: --offer-duration must be one of:",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			csvPath := filepath.Join(t.TempDir(), "offers.csv")
			if err := os.WriteFile(csvPath, []byte(test.body), 0o600); err != nil {
				t.Fatalf("WriteFile() error: %v", err)
			}

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"subscriptions", "offers", "introductory", "import",
					"--subscription-id", "SUB_ID",
					"--input", csvPath,
				}); err != nil {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q in stderr, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestSubscriptionsIntroductoryOffersImport_InvalidRowPeriodsReturnUsage(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "non integer periods",
			body:    "territory,number_of_periods\nUSA,abc\n",
			wantErr: "row 1: number_of_periods must be a positive integer",
		},
		{
			name:    "zero periods",
			body:    "territory,number_of_periods\nUSA,0\n",
			wantErr: "row 1: number_of_periods must be a positive integer",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			csvPath := filepath.Join(t.TempDir(), "offers.csv")
			if err := os.WriteFile(csvPath, []byte(test.body), 0o600); err != nil {
				t.Fatalf("WriteFile() error: %v", err)
			}

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"subscriptions", "offers", "introductory", "import",
					"--subscription-id", "SUB_ID",
					"--input", csvPath,
				}); err != nil {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q in stderr, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestSubscriptionsIntroductoryOffersImport_InvalidRowTerritoryReturnsUsage(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "missing territory",
			body:    "territory,offer_mode\n,FREE_TRIAL\n",
			wantErr: "row 1: territory is required",
		},
		{
			name:    "unmappable territory",
			body:    "territory\nAtlantis\n",
			wantErr: `row 1: territory "Atlantis" could not be mapped to an App Store Connect territory ID`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			csvPath := filepath.Join(t.TempDir(), "offers.csv")
			if err := os.WriteFile(csvPath, []byte(test.body), 0o600); err != nil {
				t.Fatalf("WriteFile() error: %v", err)
			}

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"subscriptions", "offers", "introductory", "import",
					"--subscription-id", "SUB_ID",
					"--input", csvPath,
				}); err != nil {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q in stderr, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestSubscriptionsIntroductoryOffersImport_MissingRequiredOfferFieldsReturnUsage(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "missing offer mode after fallback",
			args: []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "SUB_ID",
				"--input", writeTempIntroOffersCSV(t, "territory\nUSA\n"),
			},
			wantErr: "row 1: offer_mode is required",
		},
		{
			name: "missing offer duration after fallback",
			args: []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "SUB_ID",
				"--input", writeTempIntroOffersCSV(t, "territory,offer_mode\nUSA,FREE_TRIAL\n"),
			},
			wantErr: "row 1: offer_duration is required",
		},
		{
			name: "missing periods after fallback",
			args: []string{
				"subscriptions", "offers", "introductory", "import",
				"--subscription-id", "SUB_ID",
				"--input", writeTempIntroOffersCSV(t, "territory,offer_mode,offer_duration\nUSA,FREE_TRIAL,ONE_WEEK\n"),
			},
			wantErr: "row 1: number_of_periods is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
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
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected %q in stderr, got %q", test.wantErr, stderr)
			}
		})
	}
}

func writeTempIntroOffersCSV(t *testing.T, body string) string {
	t.Helper()

	csvPath := filepath.Join(t.TempDir(), "offers.csv")
	if err := os.WriteFile(csvPath, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	return csvPath
}

func TestSubscriptionsIntroductoryOffersImport_DryRunUsesDefaultsWithoutMutations(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})

	csvPath := writeTempIntroOffersCSV(t, "territory\nUSA\nAfghanistan\n")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	type importSummary struct {
		DryRun  bool `json:"dryRun"`
		Total   int  `json:"total"`
		Created int  `json:"created"`
		Failed  int  `json:"failed"`
	}

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"subscriptions", "offers", "introductory", "import",
			"--subscription-id", "SUB_ID",
			"--input", csvPath,
			"--offer-duration", "one_week",
			"--offer-mode", "free_trial",
			"--number-of-periods", "1",
			"--dry-run",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var summary importSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse JSON summary: %v", err)
	}
	if !summary.DryRun {
		t.Fatalf("expected dryRun=true")
	}
	if summary.Total != 2 || summary.Created != 2 || summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}
