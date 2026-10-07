# Auth logout respects ASC_BYPASS_KEYCHAIN

Changed in 5.10.1. When `ASC_BYPASS_KEYCHAIN` is set to 1/true/yes/on,
`asc auth logout --all` (and `--all --include-global`, or logout with no
`--name`) removes credentials only from config files. Earlier releases also
deleted every `asc` credential from the system keychain, even though every
other auth command skips the keychain while the variable is set. As a result,
`asc auth logout --all --confirm` in a CI job, sandbox, agent, or test run that
set `ASC_BYPASS_KEYCHAIN=1` deleted the developer's own keychain credentials.

`asc auth logout --name` already skipped the keychain under the bypass and is
unchanged. Behavior without `ASC_BYPASS_KEYCHAIN` is unchanged.

While the bypass is set, logout says so on stderr:

```text
Note: ASC_BYPASS_KEYCHAIN is set, so auth logout changes config files only and leaves keychain entries untouched.
```

The command still exits 0 and prints the same stdout as before. To remove
keychain credentials, run `asc auth logout` with `ASC_BYPASS_KEYCHAIN` unset.

`asc ads auth logout` and `asc storekit auth logout` keep their own variables,
`ASC_ADS_BYPASS_KEYCHAIN` and `ASC_STOREKIT_BYPASS_KEYCHAIN`, which already
applied to every keychain operation in those stores.
