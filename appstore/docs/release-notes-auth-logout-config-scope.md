# Auth logout respects ASC_CONFIG_PATH

Changed in 5.9.1. When `ASC_CONFIG_PATH` is set, `asc auth logout` (with
`--name`, `--all`, or neither) removes credentials only from that file and the
keychain. Earlier releases also removed matching credentials from
`~/.asc/config.json`, even though no command reads that file while
`ASC_CONFIG_PATH` is set. As a result, `asc auth logout --all --confirm` in an
isolated CI job, sandbox, agent, or test run also deleted the developer's own
global credentials.

A keychain login that replaces a profile follows the same rule: while
`ASC_CONFIG_PATH` is set, it removes the replaced profile's config entry only
from that file.

Keychain cleanup is unchanged, because the keychain is not scoped by
`ASC_CONFIG_PATH`. Behavior without `ASC_CONFIG_PATH` is unchanged: logout
removes matching credentials from the active config file and
`~/.asc/config.json`.

When logout leaves matching credentials in `~/.asc/config.json`, it says so on
stderr instead of removing them silently:

```text
Warning: ASC_CONFIG_PATH is set, so auth logout did not change /Users/you/.asc/config.json, which still holds credentials named 'MyKey'; to remove them, run auth logout with ASC_CONFIG_PATH unset or pass --include-global.
```

The command still exits 0 and prints the same stdout as before. If
`--name` matches a profile only in `~/.asc/config.json`, nothing in scope
matches, so logout prints the warning and then fails with the existing
not-found error (exit 1), where earlier releases removed the global profile.
The same not-found error now applies when the active config file does not
exist; earlier releases reported success there without removing anything.

## Migration

To keep the earlier cross-file cleanup, pass the new `--include-global` flag:

```bash
asc auth logout --all --include-global --confirm
asc auth logout --name "MyKey" --include-global --confirm
```

Or run logout with `ASC_CONFIG_PATH` unset, which cleans the active config file
and `~/.asc/config.json` as before.

Releases before 5.9.1 could write a keychain-bypassing login to
`~/.asc/config.json` even when `ASC_CONFIG_PATH` was set (see
`docs/release-notes-auth-login-config-path.md`). The earlier logout behavior
also removed those misplaced profiles. If you relied on that, check the warning
output and use `--include-global` once to remove them.
