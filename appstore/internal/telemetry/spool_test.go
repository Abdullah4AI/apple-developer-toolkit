package telemetry

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const spoolWriterHelperEnv = "ASC_TEST_TELEMETRY_SPOOL_WRITER"

func TestSpoolEvictsOldestRecordsByCount(t *testing.T) {
	store := testSpoolStore(filepath.Join(t.TempDir(), "telemetry-spool.jsonl"))
	store.maxRecords = 3

	for i := range 5 {
		if err := store.append(testSpoolRecord(fmt.Sprintf("event-%02d", i))); err != nil {
			t.Fatalf("append event %d: %v", i, err)
		}
	}

	assertSpoolEventIDs(t, store, "event-02", "event-03", "event-04")
}

func TestSpoolEvictsOldestRecordsByEncodedBytes(t *testing.T) {
	store := testSpoolStore(filepath.Join(t.TempDir(), "telemetry-spool.jsonl"))
	recordSize := len(mustEncodeSpoolRecord(t, testSpoolRecord("event-00")))
	store.maxBytes = recordSize * 2

	for i := range 3 {
		if err := store.append(testSpoolRecord(fmt.Sprintf("event-%02d", i))); err != nil {
			t.Fatalf("append event %d: %v", i, err)
		}
	}

	assertSpoolEventIDs(t, store, "event-01", "event-02")
}

func TestSpoolRejectsOversizedNewestWithoutEvictingQueue(t *testing.T) {
	store := testSpoolStore(filepath.Join(t.TempDir(), "telemetry-spool.jsonl"))
	existing := testSpoolRecord("event-old")
	if err := store.append(existing); err != nil {
		t.Fatalf("append existing event: %v", err)
	}

	existingSize := len(mustEncodeSpoolRecord(t, existing))
	store.maxBytes = existingSize + 16
	store.maxRecordBytes = store.maxBytes * 4
	oversized := testSpoolRecord("event-new")
	oversized.Event.ASCVersion = strings.Repeat("x", store.maxBytes)

	if err := store.append(oversized); !errors.Is(err, errSpoolRecordTooLarge) {
		t.Fatalf("append oversized event error = %v, want %v", err, errSpoolRecordTooLarge)
	}
	assertSpoolEventIDs(t, store, "event-old")
}

func TestSpoolLockAlwaysAttemptsUncontendedLock(t *testing.T) {
	store := testSpoolStore(filepath.Join(t.TempDir(), "telemetry-spool.jsonl"))
	store.lockWait = time.Nanosecond

	if err := store.append(testSpoolRecord("event-01")); err != nil {
		t.Fatalf("append with uncontended lock error = %v", err)
	}
	assertSpoolEventIDs(t, store, "event-01")
}

func TestSpoolRecoversCorruptAndTruncatedRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telemetry-spool.jsonl")
	store := testSpoolStore(path)
	if err := os.WriteFile(path, append(mustEncodeSpoolRecord(t, testSpoolRecord("event-01")), []byte("{\"event\":")...), 0o600); err != nil {
		t.Fatalf("write truncated spool: %v", err)
	}
	crashTempPath := path + ".crash.tmp"
	if err := os.WriteFile(crashTempPath, mustEncodeSpoolRecord(t, testSpoolRecord("event-temp")), 0o600); err != nil {
		t.Fatalf("write abandoned temp file: %v", err)
	}

	if err := store.append(testSpoolRecord("event-02")); err != nil {
		t.Fatalf("append after truncated record: %v", err)
	}
	assertSpoolEventIDs(t, store, "event-01", "event-02")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read repaired spool: %v", err)
	}
	if strings.Contains(string(data), "{\"event\":\n") || strings.Contains(string(data), "event-temp") {
		t.Fatalf("repaired spool retained crash debris: %q", data)
	}
	if _, err := os.Stat(crashTempPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("crash temp stat error = %v, want not exist after recovery", err)
	}
}

func TestSpoolConcurrentProcessWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telemetry-spool.jsonl")
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	type process struct {
		id     string
		cmd    *exec.Cmd
		output *bytes.Buffer
	}
	processes := make([]process, 0, 12)
	for i := range 12 {
		id := fmt.Sprintf("event-%02d", i)
		cmd := exec.Command(executable, "-test.run=^TestSpoolWriterHelperProcess$")
		cmd.Env = append(
			os.Environ(),
			spoolWriterHelperEnv+"=1",
			"ASC_TEST_TELEMETRY_SPOOL_PATH="+path,
			"ASC_TEST_TELEMETRY_EVENT_ID="+id,
		)
		output := &bytes.Buffer{}
		cmd.Stdout = output
		cmd.Stderr = output
		if err := cmd.Start(); err != nil {
			t.Fatalf("start writer %s: %v", id, err)
		}
		processes = append(processes, process{id: id, cmd: cmd, output: output})
	}
	for _, process := range processes {
		if err := process.cmd.Wait(); err != nil {
			t.Fatalf("writer %s failed: %v\n%s", process.id, err, process.output)
		}
	}

	store := testSpoolStore(path)
	records, err := store.snapshot(0)
	if err != nil {
		t.Fatalf("snapshot() error = %v", err)
	}
	if len(records) != len(processes) {
		t.Fatalf("spool records = %d, want %d", len(records), len(processes))
	}
	seen := make(map[string]bool, len(records))
	for _, record := range records {
		seen[record.Event.EventID] = true
	}
	for _, process := range processes {
		if !seen[process.id] {
			t.Errorf("missing concurrently written event %q", process.id)
		}
	}
}

func TestSpoolWriterHelperProcess(t *testing.T) {
	if os.Getenv(spoolWriterHelperEnv) != "1" {
		return
	}
	store := testSpoolStore(os.Getenv("ASC_TEST_TELEMETRY_SPOOL_PATH"))
	// Twelve writers queue on one lock, so a slow runner can spend the whole
	// production budget waiting. This test checks that every append lands
	// intact, not how long the queue takes; the deadline only stops a hang.
	store.lockWait = time.Hour
	if err := store.append(testSpoolRecord(os.Getenv("ASC_TEST_TELEMETRY_EVENT_ID"))); err != nil {
		t.Fatalf("append helper record: %v", err)
	}
}

func testSpoolStore(path string) spoolStore {
	return spoolStore{
		path:           path,
		maxRecords:     maxSpoolRecords,
		maxBytes:       maxSpoolBytes,
		maxRecordBytes: maxSpoolRecordBytes,
		lockWait:       lockTimeout,
	}
}

func testSpoolRecord(eventID string) spoolRecord {
	return spoolRecord{
		Event: Event{
			EventID:       eventID,
			SchemaVersion: 3,
			ASCVersion:    "1.2.3",
			CommandPath:   "asc builds list",
		},
		Endpoint: "https://telemetry.example.test/events",
	}
}

func mustEncodeSpoolRecord(t *testing.T, record spoolRecord) []byte {
	t.Helper()
	data, err := encodeSpoolRecord(record)
	if err != nil {
		t.Fatalf("encodeSpoolRecord() error = %v", err)
	}
	return data
}

func assertSpoolEventIDs(t *testing.T, store spoolStore, want ...string) {
	t.Helper()
	records, err := store.snapshot(0)
	if err != nil {
		t.Fatalf("snapshot() error = %v", err)
	}
	got := make([]string, 0, len(records))
	for _, record := range records {
		got = append(got, record.Event.EventID)
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("spool event IDs = %v, want %v", got, want)
	}
}
