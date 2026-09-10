# Environment sessions on supported web reads

Expose `--session-from-env` only on `web removed-apps list`, `web api-keys list`,
and `web api-keys view`. Their help states that `ASC_WEB_SESSION` is used in
memory without persistence. The option does not read or write the session cache
or keychain, log in, renew a session, or switch providers. Other commands reject
the flag with a usage error before accessing credentials.

## Compatibility

This change replaces the unreleased global environment-session option introduced
by commit `6df53c8c2`. That commit is after the `5.0.0` tag, and the global option
is absent from the published 5.0.0 CLI. The requested replacement therefore does
not remove a released command-line contract and does not retain an alias for the
unreleased spelling. Existing cached-session behavior and explicit session import
remain unchanged.

## Validation

Command tests cover the exact three-command scope, supported GET requests,
rejected commands and provider or 2FA overrides, invalid and expired bundles,
identity mismatches, explicit opt-in, credential-free help, and unchanged session
storage. HTTP endpoints and output formats are unchanged; the existing fixture
contracts remain applicable.
