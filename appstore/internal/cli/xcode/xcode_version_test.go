package xcode

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	localxcode "github.com/Abdullah4AI/apple-developer-toolkit/appstore/internal/xcode"
)

func TestXcodeVersionViewCommandOutputsResult(t *testing.T) {
	originalRunGetVersion := runGetVersion
	t.Cleanup(func() {
		runGetVersion = originalRunGetVersion
	})

	runGetVersion = func(ctx context.Context, projectDir, target string) (*localxcode.VersionInfo, error) {
		return &localxcode.VersionInfo{
			Version:     "1.2.3",
			BuildNumber: "42",
			ProjectDir:  projectDir,
			Target:      target,
			Modern:      true,
		}, nil
	}

	stdout, stderr, err := runXcodeVersionCommand(t, []string{"view", "--project-dir", "./MyApp", "--target", "App", "--output", "json"})
	if err != nil {
		t.Fatalf("view run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected view stderr to be empty, got %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output from view command")
	}
}

func TestXcodeVersionViewCommandSupportsProjectFlag(t *testing.T) {
	originalRunGetVersion := runGetVersion
	t.Cleanup(func() {
		runGetVersion = originalRunGetVersion
	})

	runGetVersion = func(ctx context.Context, projectDir, target string) (*localxcode.VersionInfo, error) {
		if projectDir != "./MyApp/App.xcodeproj" {
			t.Fatalf("expected explicit project path, got %q", projectDir)
		}
		return &localxcode.VersionInfo{
			Version:     "1.2.3",
			BuildNumber: "42",
			ProjectDir:  "./MyApp",
			Target:      target,
			Modern:      true,
		}, nil
	}

	stdout, stderr, err := runXcodeVersionCommand(t, []string{
		"view",
		"--project", "./MyApp/App.xcodeproj",
		"--target", "App",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("view run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected view stderr to be empty, got %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output from view command")
	}
}

func TestXcodeVersionEditCommandOutputsResult(t *testing.T) {
	originalRunSetVersion := runSetVersion
	t.Cleanup(func() {
		runSetVersion = originalRunSetVersion
	})

	runSetVersion = func(ctx context.Context, opts localxcode.SetVersionOptions) (*localxcode.SetVersionResult, error) {
		return &localxcode.SetVersionResult{
			Version:     opts.Version,
			BuildNumber: opts.BuildNumber,
			ProjectDir:  opts.ProjectDir,
		}, nil
	}

	stdout, stderr, err := runXcodeVersionCommand(t, []string{"edit", "--project-dir", "./MyApp", "--version", "1.3.0", "--build-number", "42", "--output", "json"})
	if err != nil {
		t.Fatalf("edit run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected edit stderr to be empty, got %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output from edit command")
	}
}

func TestXcodeVersionEditCommandSupportsProjectFlag(t *testing.T) {
	originalRunSetVersion := runSetVersion
	t.Cleanup(func() {
		runSetVersion = originalRunSetVersion
	})

	runSetVersion = func(ctx context.Context, opts localxcode.SetVersionOptions) (*localxcode.SetVersionResult, error) {
		if opts.ProjectDir != "./MyApp/App.xcodeproj" {
			t.Fatalf("expected explicit project path, got %q", opts.ProjectDir)
		}
		return &localxcode.SetVersionResult{
			Version:     opts.Version,
			BuildNumber: opts.BuildNumber,
			ProjectDir:  "./MyApp",
		}, nil
	}

	stdout, stderr, err := runXcodeVersionCommand(t, []string{
		"edit",
		"--project", "./MyApp/App.xcodeproj",
		"--version", "1.3.0",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("edit run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected edit stderr to be empty, got %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output from edit command")
	}
}

func TestXcodeVersionBumpCommandSupportsTargetFlag(t *testing.T) {
	originalRunBumpVersion := runBumpVersion
	t.Cleanup(func() {
		runBumpVersion = originalRunBumpVersion
	})

	runBumpVersion = func(ctx context.Context, opts localxcode.BumpVersionOptions) (*localxcode.BumpVersionResult, error) {
		if opts.ProjectDir != "./MyApp/App.xcodeproj" {
			t.Fatalf("expected explicit project path, got %q", opts.ProjectDir)
		}
		if opts.Target != "Extension" {
			t.Fatalf("expected bump target Extension, got %q", opts.Target)
		}
		return &localxcode.BumpVersionResult{
			BumpType:   string(opts.BumpType),
			OldVersion: "2.0.0",
			NewVersion: "2.0.1",
			ProjectDir: opts.ProjectDir,
		}, nil
	}

	stdout, stderr, err := runXcodeVersionCommand(t, []string{
		"bump",
		"--project", "./MyApp/App.xcodeproj",
		"--target", "Extension",
		"--type", "patch",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("bump run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected bump stderr to be empty, got %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output from bump command")
	}
}

func TestXcodeVersionEditCommandOmitsTargetFlag(t *testing.T) {
	if xcodeVersionEditCommand().FlagSet.Lookup("target") != nil {
		t.Fatal("expected edit command to omit --target")
	}
}

func TestXcodeVersionCommandsExposeProjectFlag(t *testing.T) {
	if xcodeVersionViewCommand().FlagSet.Lookup("project") == nil {
		t.Fatal("expected view command to expose --project")
	}
	if xcodeVersionEditCommand().FlagSet.Lookup("project") == nil {
		t.Fatal("expected edit command to expose --project")
	}
	if xcodeVersionBumpCommand().FlagSet.Lookup("project") == nil {
		t.Fatal("expected bump command to expose --project")
	}
}

func TestXcodeVersionBumpCommandExposesTargetFlag(t *testing.T) {
	if xcodeVersionBumpCommand().FlagSet.Lookup("target") == nil {
		t.Fatal("expected bump command to expose --target")
	}
}

func runXcodeVersionCommand(t *testing.T, args []string) (string, string, error) {
	t.Helper()

	cmd := XcodeVersionCommand()
	var runErr error
	stdout, stderr := captureXcodeVersionOutput(t, func() {
		if err := cmd.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = cmd.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func captureXcodeVersionOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe error: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe error: %v", err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW

	stdoutCh := make(chan string, 1)
	stderrCh := make(chan string, 1)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stdoutR)
		_ = stdoutR.Close()
		stdoutCh <- buf.String()
	}()
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stderrR)
		_ = stderrR.Close()
		stderrCh <- buf.String()
	}()

	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = stdoutW.Close()
		_ = stderrW.Close()
	}()

	fn()

	_ = stdoutW.Close()
	_ = stderrW.Close()

	return <-stdoutCh, <-stderrCh
}
