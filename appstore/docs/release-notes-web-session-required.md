# Clearer error when no Apple web session is available

Changed in 5.9.0. Some `asc web` commands need a signed-in Apple Account
session that they cannot create. This happens when:

- nothing is cached and no account is selected, or
- the selected account has no usable cached session, and `ASC_WEB_PASSWORD`,
  a saved password, and an interactive terminal are all unavailable.

Every web command except `asc web auth login` now prints a short error and a
`Hint:` line with the next step in that case, instead of the command's full
usage page:

```text
Error: no Apple web session is cached
Hint: asc web commands need a signed-in Apple Account session, and signing in needs an interactive terminal for the password and two-factor code. Run 'asc web auth login --apple-id EMAIL' in a terminal, or load a session exported elsewhere with 'asc web auth import --file FILE'. Unattended sign-in needs ASC_WEB_PASSWORD and ASC_WEB_2FA_CODE_COMMAND, plus --apple-id or ASC_WEB_APPLE_ID.
```

When an account was selected, the error names it. Where the public App Store
Connect API can answer instead, the hint also names that command. For example,
`asc web review show` points to `asc review status --app APP_ID`.

The exit code is unchanged: these failures still exit with usage code `2`.
`asc web auth login` keeps its existing usage errors and usage page.
