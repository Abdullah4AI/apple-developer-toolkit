# Go Standards

Follow idiomatic Go so the code is predictable to anyone who reads Go.

## Formatting

- Always run `gofmt` (and `gofumpt` via `make format`)
- No manual formatting

## Naming

- Use mixedCaps (not snake_case)
- Keep common initialisms uppercase: `ID`, `URL`, `API`, `JSON`

## Error Handling

- Return errors, don't panic for expected failures
- Wrap with context using `%w`: `fmt.Errorf("operation failed: %w", err)`

## Context

- Pass `context.Context` into network operations
- Respect timeouts and cancellations

## Types

- Model request/response types with JSON tags
- Use pointers for optional fields, values for required fields
- Prefer typed `const` values (not raw strings) for API enums and resource types

## CLI Behavior

- If a flag is accepted, it must be implemented or error
- Never silently ignore flags
- Data goes to stdout, errors to stderr
- Keep JSON minified by default
- If a command already prints structured output and must exit non-zero,
  return `cmd.NewReportedError(err)` to avoid duplicate stderr logging

## Dependencies

- Standard library first
- Avoid new deps unless necessary and justified

## CLI Help Output

- Use `UsageFunc` on ffcli commands for consistent help formatting
- When returning `flag.ErrHelp`, do **not** call `fs.Usage()` manually (avoids duplicate output)
- Help output is written to stderr by default

## Testing

### Error Checking

- **Do**: Use `errors.Is()` or typed extraction via `errors.AsType()` (Go 1.26+)
- **Do**: Use `errors.As()` when you need compatibility with older Go versions
- **Do**: Simply check `err != nil` when only verifying an error occurred
- **Don't**: Match error strings with `strings.Contains(err.Error(), "...")` - this is fragile

```go
// Good: Check for specific error type (Go 1.26+)
if notFoundErr, ok := errors.AsType[*NotFoundError](err); ok {
    // handle not found
    _ = notFoundErr
}

// Also valid: classic errors.As pattern
var legacyErr *NotFoundError
if errors.As(err, &legacyErr) {
    // handle not found
}

// Good: Just verify error occurred
if err == nil {
    t.Fatal("expected error, got nil")
}
t.Logf("got expected error: %v", err)

// Bad: Fragile string matching
if !strings.Contains(err.Error(), "not found") {  // DON'T DO THIS
    t.Fatal("wrong error")
}
```

### Empty Results

- Prefer empty arrays/objects over `null` when no data is returned
- CLI commands should exit `0` for empty results and render empty output cleanly
- Add tests for empty responses (including empty pagination)

### Test Isolation

- Use `t.TempDir()` for temporary files
- Use `t.ArtifactDir()` for test artifacts you want to inspect with `go test -artifacts`
- Use `t.Setenv()` to set environment variables (auto-cleaned up)
- Isolate from user config by setting `ASC_CONFIG_PATH` to a temp path
