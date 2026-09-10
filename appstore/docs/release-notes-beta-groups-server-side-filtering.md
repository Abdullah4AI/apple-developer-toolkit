# TestFlight beta groups server-side filtering release note

`asc testflight groups list --app APP_ID --internal` and `--external` are now
filtered by App Store Connect instead of in the CLI. `GET /v1/apps/{id}/betaGroups`
accepts only `limit` and `fields[betaGroups]`, so the command previously fetched every page and
filtered the aggregate in Go. Those requests now go to `GET /v1/betaGroups`
with `filter[app]` and `filter[isInternalGroup]`, while retaining the complete
multi-page result.

Two new flags cover query parameters that endpoint already
documents: `--name` filters on the exact group name (`filter[name]`) and
`--sort` accepts `name`, `-name`, `createdDate`, `-createdDate`, `publicLinkEnabled`,
`-publicLinkEnabled`, `publicLinkLimit`, and `-publicLinkLimit`. An unsupported
`--sort` value is a usage error listing the valid values.

## Pagination compatibility

App-scoped `--internal` and `--external` listings continue to collect every
matching page automatically when `--name` and `--sort` are absent, so existing
invocations retain complete output:

```bash
asc testflight groups list --app APP_ID --internal
```

Combined app-scoped filters, including `--internal --name NAME`, follow the
standard one-page default; add `--paginate` to collect all matching pages.
Listings that use only the new `--name` or `--sort` flags behave the same way.
Global listings are unchanged and also require `--paginate` for aggregation.

For the stable app-scoped `--internal` and `--external` paths, `--limit` remains
a cap on the final aggregate after every page is fetched. Without explicit
`--paginate`, the aggregate fetch uses the maximum page size of 200 regardless
of `--limit`, then applies the same final cap. Explicit `--paginate` uses the
same page size of 200. Other filtered and global listings retain their standard
page-size semantics for `--limit`.

`--next` now rejects `--internal`, `--external`, `--name`, and `--sort`. A
`links.next` URL is followed verbatim and already carries the query it came
from, so those flags were previously accepted and discarded. Drop them from the
follow-up call; a bare `--next`, with or without `--paginate`, is unchanged.

Unfiltered app-scoped listing (`--app APP_ID` with no filter or sort) still uses
`GET /v1/apps/{id}/betaGroups` and is unchanged.
