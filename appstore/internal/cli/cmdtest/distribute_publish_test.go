package cmdtest

import (
	"strings"
	"testing"
)

func TestDistributePublishCommandSurfaceIsAgentDiscoverable(t *testing.T) {
	root := RootCommand("1.2.3")
	distribute := findSubcommand(root, "distribute")
	if distribute == nil {
		t.Fatalf("unexpected distribute command: %#v", distribute)
	}
	publish := findSubcommand(root, "distribute", "publish")
	if publish == nil {
		t.Fatalf("unexpected distribute publish command: %#v", publish)
	}
	verifyTimeoutUsage := ""
	for _, name := range []string{
		"bundle-dir", "endpoint", "region", "bucket", "prefix", "download-endpoint", "addressing-style", "access",
		"public-base-url", "url-ttl", "download-grace", "verify-timeout", "receipt", "link-path", "output", "pretty",
	} {
		item := publish.FlagSet.Lookup(name)
		if item == nil {
			t.Fatalf("missing --%s", name)
		}
		if name == "verify-timeout" {
			verifyTimeoutUsage = item.Usage
		}
	}
	if !strings.Contains(verifyTimeoutUsage, "ASC_UPLOAD_TIMEOUT") {
		t.Fatalf("--verify-timeout usage = %q, want IPA upload-timeout guidance", verifyTimeoutUsage)
	}
}

func TestDistributePublishInvalidValueIsUsageExit(t *testing.T) {
	assertUsageExit(
		t,
		[]string{"distribute", "publish", "--bundle-dir", "bundle", "--endpoint", "http://insecure.example", "--region", "auto", "--bucket", "bucket", "--prefix", "app"},
		"--endpoint: endpoint must be an HTTPS origin",
	)
}

func TestDistributePublishRejectsOverflowingPrivateLifetimes(t *testing.T) {
	for _, test := range []struct {
		name   string
		values []string
		want   string
	}{
		{name: "overflowing sum", values: []string{"--url-ttl", "2562047h", "--download-grace", "100h"}, want: "--url-ttl must not exceed 7d"},
		{name: "maximum URL TTL", values: []string{"--url-ttl", "2562047h47m16.854775807s"}, want: "--url-ttl must not exceed 7d"},
		{name: "maximum download grace", values: []string{"--download-grace", "2562047h47m16.854775807s"}, want: "--download-grace must not exceed 7d"},
		{name: "one nanosecond over combined cap", values: []string{"--url-ttl", "168h", "--download-grace", "1ns"}, want: "--url-ttl plus --download-grace must not exceed 7d"},
		{name: "zero URL TTL", values: []string{"--url-ttl", "0s"}, want: "--url-ttl must be positive"},
		{name: "negative URL TTL", values: []string{"--url-ttl", "-1ns"}, want: "--url-ttl must be positive"},
		{name: "negative download grace", values: []string{"--download-grace", "-1ns"}, want: "--download-grace must not be negative"},
	} {
		t.Run(test.name, func(t *testing.T) {
			arguments := []string{
				"distribute", "publish",
				"--bundle-dir", "bundle", "--endpoint", "https://objects.example.com", "--region", "auto",
				"--bucket", "bucket", "--prefix", "app", "--receipt", "receipt.json", "--link-path", "link.json",
				"--url-ttl", "1h", "--download-grace", "1h",
			}
			arguments = append(arguments, test.values...)
			assertUsageExit(t, arguments, test.want)
		})
	}
}

func TestDistributePublishBoundaryValidationIsActionable(t *testing.T) {
	base := []string{
		"distribute", "publish", "--bundle-dir", "bundle", "--endpoint", "https://objects.example.com",
		"--region", "auto", "--bucket", "bucket", "--prefix", "app",
	}
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{
			name: "private-only lifetime with public access",
			args: []string{"--access", "public", "--public-base-url", "https://downloads.example.com", "--url-ttl", "1h", "--receipt", "receipt.json", "--link-path", "link.json"},
			want: "--url-ttl, --download-grace, and --download-endpoint are only valid with --access private",
		},
		{
			name: "addressing style",
			args: []string{"--addressing-style", "automatic", "--receipt", "receipt.json", "--link-path", "link.json"},
			want: "--addressing-style must be path or virtual",
		},
		{
			name: "required artifacts",
			want: "--receipt and --link-path are required",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, append(append([]string(nil), base...), test.args...), test.want)
		})
	}
}
