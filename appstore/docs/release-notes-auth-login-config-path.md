# Keychain-bypassing login writes the ASC_CONFIG_PATH file

Changed in 5.9.1. When `ASC_CONFIG_PATH` is set, `asc auth login` with
`--bypass-keychain` or a truthy `ASC_BYPASS_KEYCHAIN` now stores credentials in
that file. Earlier releases wrote `~/.asc/config.json` instead, even though every
command reads only the `ASC_CONFIG_PATH` file when it is set. The login reported
success, but later commands in the same environment could not find the profile,
and the credentials landed in the global config the variable was meant to
isolate from.

`asc auth init` follows the same rule: without `--local`, it writes its template
to the `ASC_CONFIG_PATH` file when set.

Behavior without `ASC_CONFIG_PATH` is unchanged: a keychain-bypassing login and
`asc auth init` write `~/.asc/config.json`, and `--local` writes
`./.asc/config.json`. Keychain logins are unchanged.

An explicit `--local` still writes `./.asc/config.json` when `ASC_CONFIG_PATH`
is set. Because later commands read only the `ASC_CONFIG_PATH` file, the command
now prints a warning to stderr:

```text
Warning: ASC_CONFIG_PATH is set, so other commands read /path/to/config.json instead of /repo/.asc/config.json; unset ASC_CONFIG_PATH or point it at /repo/.asc/config.json to use this file.
```

## Migration

If a script set `ASC_CONFIG_PATH`, logged in with the keychain bypassed, and then
read the credentials from `~/.asc/config.json` with `ASC_CONFIG_PATH` unset, run
that login with `ASC_CONFIG_PATH` unset. Credentials an earlier release wrote to
`~/.asc/config.json` stay there; check that file and remove any profile you did
not mean to keep.
