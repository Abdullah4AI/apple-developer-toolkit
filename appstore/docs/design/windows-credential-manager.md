# Windows Credential Manager persistence

Issue [2933](https://github.com/rorkai/App-Store-Connect-CLI/issues/2933) requests
native Windows credential storage. `WinCredBackend` is already enabled, using
the pinned keyring dependency and existing `keyring:asc:asc:credential:<name>`
targets. A second backend or renamed namespace would duplicate existing support
and strand stored profiles.

The reproduced defect is legacy migration: only macOS has a distinct named
`asc` keychain. Windows and Linux backends ignore `KeychainName`, so opening the
macOS legacy configuration aliases the current store. A normal credential lookup
enters legacy migration, removes the supposed duplicate, and deletes the current
credential. The regression reaches RED on the first lookup with an aliased
backend; it asserts repeated resolution and listing retain the secure entry.

Limit legacy-store opening to macOS and treat an unavailable legacy store as empty
during named logout, while preserving real legacy-listing errors. Retain native
backend selection, credential
targets, config fallback, explicit bypass flags, and macOS migration. Login stores
the private key payload securely; config can retain default selection and public
metadata. Re-login without bypass migrates a matching config-backed profile only
after the secure write succeeds. No automatic migration occurs during diagnosis.

Local regression tests simulate backends that ignore the macOS keychain name and
preserve the macOS named-store configuration. Windows CI additionally runs an
explicit native smoke with synthetic P-256 keys and unique owned profiles through
the built CLI in fresh processes. It checks native login, config-to-native
re-login, repeated status and JWT generation after deleting the input key file,
profile switching, and named logout without deleting another fixture. JWT
signatures are verified locally; no App Store Connect request is made. Cleanup
removes only the smoke test's credential targets. Native acceptance requires the
Windows CI result; cross-compilation alone does not prove it.
