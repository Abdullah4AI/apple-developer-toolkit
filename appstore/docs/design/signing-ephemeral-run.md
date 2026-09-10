# Ephemeral signing command environment

## Placement and current behavior

`asc signing` can fetch certificates and provisioning profiles and can sync
public signing material through an encrypted Git store. It cannot currently
make a distribution identity available to one local build without importing
that identity into a persistent user keychain. `asc profiles local install`
similarly installs a profile persistently. Agents therefore have to mutate
long-lived host state and invent their own cleanup.

This change adds a local execution boundary under the existing
signing command group:

```text
asc signing run \
  --identity .asc/signing/distribution.p12 \
  --identity-password-file .asc/secrets/p12-password \
  --profile .asc/signing/App.mobileprovision \
  --purpose release-testing \
  --receipt .asc/distribution/signing-run.json \
  -- xcodebuild -exportArchive ...
```

`--identity` and `--profile` are required. `--purpose` defaults to, and only
accepts, `release-testing` in this first slice. An identity password is read
from `--identity-password-file`; an omitted flag means an empty P12 password.
The password is never accepted inline. Arguments after `--` are executed
directly without a shell. The child inherits stdin, stdout, and stderr.

## Validation and security boundary

All input validation completes before local signing state changes:

- all input files are opened without following a final symlink, are size
  bounded, and have owner-safe permissions (the identity and password must not
  be accessible by group or other users);
- the PKCS#12 must contain exactly one usable private key and leaf certificate,
  and their public keys must match;
- the profile must decode as a signed mobile provisioning profile, be unexpired,
  target iOS, contain a structurally valid exact or terminal-wildcard bundle
  pattern and registered devices, and not be an enterprise or development
  profile (exact target matching remains the exporter/orchestrator's job);
- the profile must embed the identity certificate, and its team identifier must
  match the certificate organizational unit.

The command creates an unpredictable mode-0700 temporary directory and a
dedicated keychain with an in-memory random password. The identity import is
performed through Security.framework rather than a command-line password and is
restricted to `/usr/bin/codesign`; `security -A`, the login keychain, and the
default keychain are never used. The imported certificate, private key, and an
exact-fingerprint codesign operation are verified before child execution. A
cross-process lock serializes the short period in which the temporary keychain
is appended to the user search list. Cleanup rereads the current list and
removes only ASC's exact temporary entry, preserving unrelated concurrent
changes, before deleting the temporary keychain.

Mutable input buffers for the source PKCS#12, password file, profile, normalized
PKCS#12, generated passwords, and partition-list stdin are cleared on every
return path after their last required use. This is best-effort lifetime
reduction, not a claim of guaranteed process-memory erasure: Go strings created
for the PKCS#12 library and copies inside Go, C, Security.framework, or the
operating system are immutable or outside ASC's direct control.

The profile is installed at Xcode's version-appropriate provisioning profile
path only if no file exists for that UUID. An identical pre-existing profile is
reused and left untouched; a different file at that path is a hard conflict.
A profile created by this command is atomically moved to a same-directory
quarantine name and its inode and digest are reverified before unlinking. A file
replaced during cleanup is restored rather than deleted. User files are never
overwritten or deleted.

A mode-0600 crash journal contains only cleanup coordinates, never credentials.
It is written before keychain creation and before publishing a staged profile.
After acquiring the per-user lock, the next run validates journal ownership,
permissions, hard-link count, containment, schema, digests, and file identities
before attempting bounded recovery. Recovery removes regular files contained by
the validated mode-0700 temporary directory, including keychain lock sidecars,
while rejecting nested directories, symlinks, and other non-regular entries.
Incomplete cleanup retains the journal and private temporary directory for the
next run.

The optional receipt is a mode-0600, no-overwrite JSON file. It records only
purpose, outcome, child exit code, certificate and profile SHA-256 digests,
profile UUID, team and bundle identifiers, and cleanup state. It never records
passwords, private keys, device identifiers, or child command arguments. Its
destination and parent are preflighted before any signing-environment or child
side effects; the completed receipt is still published atomically without
replacing a file created after that preflight.

## Output, failures, and compatibility

The command itself writes no success data to stdout so the child owns the data
stream. Setup and cleanup failures are diagnostics on stderr. Usage failures
exit 2. The wrapped child's normal exit code and signal-derived shell exit code
are preserved exactly after best-effort cleanup through a private ASC-only error
marker, so ordinary subprocess failures elsewhere retain the existing generic
ASC mapping. `SIGINT`, `SIGTERM`, and `SIGHUP` received by the wrapper are
forwarded unchanged to the running child process group before waiting for it.
Receipt write failure is a command failure when the child succeeded; a child
failure remains authoritative and the receipt failure is rendered separately
before root handling suppresses the already-inherited child diagnostic. The same
render-before-join rule applies to cleanup and lock-release failures accompanying
a child exit. Setup failures and their cleanup failures remain unreported until
the root renders their complete joined diagnostic once.

The change is additive and macOS-only. Other platforms return a validation
error before reading or changing host signing state. No current signing command
or output changes, so no migration or deprecation is needed. Release, CI, and
`make build-all` recipes explicitly enable CGO for both macOS architectures so
every distributed Darwin binary includes the Security.framework boundary;
Linux and Windows cross-builds keep CGO disabled.

## RED-GREEN and verification

Coverage starts with failing tests for required flags, invalid purposes,
missing child commands, hidden password input, command discoverability, exact
child exit propagation, P12/profile mismatch, expired and device-ineligible
profiles, process ordering, search-list restoration after every failure point,
profile reuse/conflict/removal, receipt redaction and permissions, interruption,
and concurrent runs. The focused signing and root command tests run before the
built-binary validation and repository gate.

A live smoke test uses a disposable keychain and local development fixtures; it
does not mutate App Store Connect. End-to-end validation in the distribution
stack will run an Xcode release-testing export and verify the resulting signed
IPA.

## Alternatives

Importing into the login keychain would be shorter but leaves durable host
state, creates ambiguity when multiple identities have the same name, and is
unsafe for unattended agents. Relying only on `CODE_SIGN_KEYCHAIN` or build
settings does not cover arbitrary child tools consistently. A shell command
string would make quoting and secret handling ambiguous, so the command uses an
argv boundary after `--`.
