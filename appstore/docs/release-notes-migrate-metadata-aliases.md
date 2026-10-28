# Removed `asc migrate metadata` aliases

`asc migrate metadata pull`, `asc migrate metadata push`, and
`asc migrate metadata validate` were removed in 6.0.0. They were aliases for
`asc metadata pull`, `asc metadata push`, and `asc metadata validate`.

Any `asc migrate metadata` invocation now exits with status 2 and names the
command to use instead:

```text
Error: `asc migrate metadata pull` was removed in 6.0.0; use `asc metadata pull` instead
```

## Migration

Replace the `migrate metadata` prefix with `metadata`; flags do not change:

```bash
# Before
asc migrate metadata pull --app "APP_ID" --version "1.2.3" --dir "./metadata"
asc migrate metadata push --app "APP_ID" --version "1.2.3" --dir "./metadata" --dry-run
asc migrate metadata validate --dir "./metadata"

# After
asc metadata pull --app "APP_ID" --version "1.2.3" --dir "./metadata"
asc metadata push --app "APP_ID" --version "1.2.3" --dir "./metadata" --dry-run
asc metadata validate --dir "./metadata"
```

`asc migrate import`, `asc migrate export`, and `asc migrate validate` are not
affected.
