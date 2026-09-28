//go:build !windows

package signing

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	// gitProcessTestHangGuard only bounds waits that would otherwise hang.
	// Cancellation is triggered explicitly after the helper reports readiness,
	// so slow process startup under host load cannot race the deadline.
	gitProcessTestHangGuard = 30 * time.Second
	// gitHelperBlockSeconds keeps the blocking helper and its child alive well
	// past every hang guard, so only process-group termination can end them
	// before the test gives up.
	gitHelperBlockSeconds = 300
)

func TestGitStoreCloneCancellationTerminatesProcessGroupAndWaits(t *testing.T) {
	startedPath := filepath.Join(t.TempDir(), "clone-started")
	childPIDPath := filepath.Join(t.TempDir(), "clone-child-pid")
	environmentPath := filepath.Join(t.TempDir(), "clone-environment")
	successPath := filepath.Join(t.TempDir(), "clone-success")
	configureGitHelperProcess(t, "clone", startedPath, childPIDPath, environmentPath, successPath)

	store := &GitStore{
		RepoURL:  "ssh://example.invalid/signing.git",
		LocalDir: filepath.Join(t.TempDir(), "clone"),
		Branch:   "main",
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- store.Clone(ctx, true) }()
	childPID := waitForGitHelperStart(t, "clone", startedPath, childPIDPath, done)
	cancelGitHelperOperation(t, "clone", cancel, done)
	assertGitHelperProcessExited(t, childPID)
	assertGitHelperEnvironmentWasRedacted(t, environmentPath)
	assertGitHelperDidNotReportSuccess(t, successPath)
	if err := store.Cleanup(); err != nil {
		t.Fatalf("cleanup canceled clone: %v", err)
	}
	if _, err := os.Stat(store.LocalDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled clone left temporary directory: %v", err)
	}
}

func TestGitStorePushCancellationTerminatesProcessGroupAndWaits(t *testing.T) {
	startedPath := filepath.Join(t.TempDir(), "push-started")
	childPIDPath := filepath.Join(t.TempDir(), "push-child-pid")
	environmentPath := filepath.Join(t.TempDir(), "push-environment")
	successPath := filepath.Join(t.TempDir(), "push-success")
	configureGitHelperProcess(t, "push", startedPath, childPIDPath, environmentPath, successPath)

	store := &GitStore{LocalDir: t.TempDir(), Branch: "main"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- store.CommitAndPush(ctx, "test signing update") }()
	childPID := waitForGitHelperStart(t, "push", startedPath, childPIDPath, done)
	cancelGitHelperOperation(t, "push", cancel, done)
	assertGitHelperProcessExited(t, childPID)
	assertGitHelperEnvironmentWasRedacted(t, environmentPath)
	assertGitHelperDidNotReportSuccess(t, successPath)
	if err := store.Cleanup(); err != nil {
		t.Fatalf("cleanup canceled push: %v", err)
	}
	if _, err := os.Stat(store.LocalDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled push left temporary directory: %v", err)
	}
}

// TestGitStoreGitHelperProcess is launched by the fake Git executable below.
// It deliberately starts a child so cancellation tests verify process-group
// cleanup rather than merely observing the direct Git process exit.
func TestGitStoreGitHelperProcess(t *testing.T) {
	if os.Getenv("ASC_GIT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	gitArgs := argsAfterTestSeparator(args)
	gitOperation := ""
	if len(gitArgs) > 0 {
		gitOperation = gitArgs[0]
	}
	writeGitHelperEnvironment(os.Getenv("ASC_GIT_HELPER_ENV_CAPTURE"))

	if gitOperation == os.Getenv("ASC_GIT_HELPER_BLOCK_OPERATION") {
		startedPath := os.Getenv("ASC_GIT_HELPER_STARTED")
		childPIDPath := os.Getenv("ASC_GIT_HELPER_CHILD_PID")
		child := exec.Command("sleep", strconv.Itoa(gitHelperBlockSeconds))
		if err := child.Start(); err != nil {
			os.Exit(3)
		}
		// Publish the child PID before the started marker so the test never
		// observes a handshake that is only half-written.
		if err := writeGitHelperFileAtomically(childPIDPath, strconv.Itoa(child.Process.Pid)); err != nil {
			os.Exit(4)
		}
		if err := writeGitHelperFileAtomically(startedPath, strconv.Itoa(os.Getpid())); err != nil {
			os.Exit(2)
		}
		time.Sleep(gitHelperBlockSeconds * time.Second)
		if err := writeGitHelperFileAtomically(os.Getenv("ASC_GIT_HELPER_SUCCESS"), "success"); err != nil {
			os.Exit(6)
		}
	}

	if gitOperation == "clone" && len(gitArgs) > 0 {
		cloneDestination := gitArgs[len(gitArgs)-1]
		if err := os.MkdirAll(cloneDestination, 0o755); err != nil {
			os.Exit(5)
		}
	}

	if gitOperation == "status" {
		_, _ = fmt.Fprintln(os.Stdout, " M signing-artifact")
	}
	os.Exit(0)
}

func configureGitHelperProcess(t *testing.T, operation, startedPath, childPIDPath, environmentPath, successPath string) {
	t.Helper()
	isolateGitHelperConfiguration(t)
	binDir := t.TempDir()
	writeTestExecutable(t, filepath.Join(binDir, "git"), `#!/bin/sh
set -eu
if [ "${ASC_GIT_HELPER_BLOCK_OPERATION}" != "${1:-}" ]; then
  if [ "${1:-}" = "status" ]; then
    printf '%s\n' ' M signing-artifact'
  fi
  exit 0
fi
exec "$ASC_GIT_HELPER_BINARY" -test.run=TestGitStoreGitHelperProcess -- "$@"
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("ASC_GIT_HELPER_BINARY", os.Args[0])
	t.Setenv("ASC_GIT_HELPER_PROCESS", "1")
	t.Setenv("ASC_GIT_HELPER_BLOCK_OPERATION", operation)
	t.Setenv("ASC_GIT_HELPER_STARTED", startedPath)
	t.Setenv("ASC_GIT_HELPER_CHILD_PID", childPIDPath)
	t.Setenv("ASC_GIT_HELPER_ENV_CAPTURE", environmentPath)
	t.Setenv("ASC_GIT_HELPER_SUCCESS", successPath)
	t.Setenv("ASC_SIGNING_SYNC_PASSWORD", "must-not-reach-git")
	// A non-empty override avoids the local SSH configuration probe; the fake
	// executable is the transport boundary under test.
	t.Setenv("GIT_SSH_COMMAND", "true")
}

func isolateGitHelperConfiguration(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	configPath := filepath.Join(home, ".gitconfig")
	contents := "[user]\n\tname = asc-gitstore-test\n\temail = gitstore-test@example.invalid\n"
	if err := writeGitHelperFileAtomically(configPath, contents); err != nil {
		t.Fatalf("write isolated gitconfig: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", configPath)
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(t.TempDir(), "missing-system-config"))
	t.Setenv("GIT_CONFIG_COUNT", "0")
}

func writeGitHelperFileAtomically(path, contents string) error {
	if path == "" {
		return errors.New("empty helper file path")
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(contents); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func argsAfterTestSeparator(args []string) []string {
	for index, arg := range args {
		if arg == "--" && index+1 < len(args) {
			return args[index+1:]
		}
	}
	return nil
}

func writeGitHelperEnvironment(path string) {
	if path == "" {
		return
	}
	_ = os.WriteFile(path, []byte(os.Getenv("ASC_SIGNING_SYNC_PASSWORD")), 0o600)
}

// waitForGitHelperStart waits until the blocking helper has published its
// child PID and started marker. The operation context has no deadline, so this
// wait tolerates arbitrarily slow startup up to the hang guard. It returns the
// child PID and registers failure-only cleanup for the helper processes.
func waitForGitHelperStart(t *testing.T, operation, startedPath, childPIDPath string, done <-chan error) int {
	t.Helper()
	deadline := time.Now().Add(gitProcessTestHangGuard)
	for {
		if data, err := os.ReadFile(startedPath); err == nil && strings.TrimSpace(string(data)) != "" {
			helperPID, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
			if parseErr != nil || helperPID <= 0 {
				t.Fatalf("parse Git %s helper PID %q: %v", operation, data, parseErr)
			}
			childPID := readGitHelperPID(t, childPIDPath)
			// Capture identities while both processes are known to be alive and
			// blocked, so failure cleanup can tell them apart from reused PIDs.
			helperIdentity := gitHelperProcessIdentity(helperPID)
			childIdentity := gitHelperProcessIdentity(childPID)
			t.Cleanup(func() {
				if t.Failed() {
					killGitHelperProcessIfUnchanged(helperPID, helperIdentity)
					killGitHelperProcessIfUnchanged(childPID, childIdentity)
				}
			})
			return childPID
		}
		select {
		case err := <-done:
			t.Fatalf("Git %s returned before its helper started: %v", operation, err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("Git %s helper did not start within %v", operation, gitProcessTestHangGuard)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// gitHelperProcessIdentity returns the start time and command line of pid.
// Together with the PID they identify one process instance; a reused PID has a
// different start time.
func gitHelperProcessIdentity(pid int) string {
	output, err := exec.Command("ps", "-o", "lstart=,command=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// killGitHelperProcessIfUnchanged kills pid only when it is still the process
// instance recorded in identity, so failure cleanup never signals a process
// that merely reused a helper's PID.
func killGitHelperProcessIfUnchanged(pid int, identity string) {
	if identity == "" || gitHelperProcessIdentity(pid) != identity {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

// cancelGitHelperOperation asserts the operation is still blocked on the
// helper, cancels its context, and waits for it to fail. The helper blocks for
// longer than the hang guard, so a return here proves cancellation ended it.
func cancelGitHelperOperation(t *testing.T, operation string, cancel context.CancelFunc, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("Git %s returned before its context was canceled: %v", operation, err)
	default:
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("canceled Git %s unexpectedly succeeded", operation)
		}
	case <-time.After(gitProcessTestHangGuard):
		t.Fatalf("Git %s did not return within %v after context cancellation", operation, gitProcessTestHangGuard)
	}
}

func readGitHelperPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(gitProcessTestHangGuard)
	var lastContents []byte
	var lastErr error
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			lastContents = data
			pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
			if parseErr == nil && pid > 0 {
				return pid
			}
			lastErr = parseErr
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			t.Fatalf("read helper PID %s: %v (last contents %q)", path, lastErr, lastContents)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func assertGitHelperProcessExited(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(gitProcessTestHangGuard)
	for {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		if err != nil {
			t.Fatalf("probe helper process %d: %v", pid, err)
		}
		if output, psErr := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output(); psErr == nil && strings.Contains(string(output), "Z") {
			return
		}
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("helper child process %d survived cancellation", pid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func assertGitHelperEnvironmentWasRedacted(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Git helper environment: %v", err)
	}
	if strings.TrimSpace(string(data)) != "" {
		t.Fatalf("Git helper received signing password environment: %q", data)
	}
}

func assertGitHelperDidNotReportSuccess(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Git helper reported success after deadline: %v", err)
	}
}

func TestReadGitHelperPIDWaitsForParseableHandshake(t *testing.T) {
	path := filepath.Join(t.TempDir(), "child-pid")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("write empty helper PID: %v", err)
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		if err := os.WriteFile(path, []byte("12345\n"), 0o600); err != nil {
			t.Errorf("write helper PID: %v", err)
		}
	}()

	pid := readGitHelperPID(t, path)
	if pid != 12345 {
		t.Fatalf("readGitHelperPID() = %d, want 12345", pid)
	}
}

func TestConfigureGitHelperProcessIsolatesGitConfiguration(t *testing.T) {
	hostHome := t.TempDir()
	hostConfig := filepath.Join(hostHome, ".gitconfig")
	if err := os.WriteFile(hostConfig, []byte("not a git config\n"), 0o600); err != nil {
		t.Fatalf("write host gitconfig: %v", err)
	}
	t.Setenv("HOME", hostHome)
	t.Setenv("GIT_CONFIG_GLOBAL", hostConfig)
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(hostHome, "system-gitconfig"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.sshCommand")
	t.Setenv("GIT_CONFIG_VALUE_0", "should-not-leak")

	configureGitHelperProcess(
		t,
		"clone",
		filepath.Join(t.TempDir(), "started"),
		filepath.Join(t.TempDir(), "child-pid"),
		filepath.Join(t.TempDir(), "environment"),
		filepath.Join(t.TempDir(), "success"),
	)

	home := os.Getenv("HOME")
	if home == "" || home == hostHome {
		t.Fatal("configureGitHelperProcess did not isolate HOME")
	}
	globalConfig := os.Getenv("GIT_CONFIG_GLOBAL")
	if globalConfig == "" || globalConfig == hostConfig {
		t.Fatal("configureGitHelperProcess did not isolate GIT_CONFIG_GLOBAL")
	}
	if os.Getenv("GIT_CONFIG_NOSYSTEM") != "1" {
		t.Fatalf("GIT_CONFIG_NOSYSTEM = %q, want 1", os.Getenv("GIT_CONFIG_NOSYSTEM"))
	}
	if os.Getenv("GIT_CONFIG_COUNT") != "0" {
		t.Fatalf("GIT_CONFIG_COUNT = %q, want 0", os.Getenv("GIT_CONFIG_COUNT"))
	}

	data, err := os.ReadFile(globalConfig)
	if err != nil {
		t.Fatalf("read isolated gitconfig: %v", err)
	}
	if strings.Contains(string(data), "not a git config") {
		t.Fatalf("isolated gitconfig reused host contents: %q", data)
	}
	if strings.TrimSpace(string(data)) == "" {
		t.Fatal("isolated gitconfig was empty")
	}
}
