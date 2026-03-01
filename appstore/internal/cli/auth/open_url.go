package auth

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
)

func openURL(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("empty URL")
	}
	parsed, err := url.Parse(target)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("invalid URL")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return fmt.Errorf("unsupported URL scheme %q", parsed.Scheme)
	}
	target = parsed.String()

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("open URL: %w", err)
	}
	return nil
}
