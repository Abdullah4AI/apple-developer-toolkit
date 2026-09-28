# Experimental signing sync remote stores

## Placement and invocation

This change extends the existing `asc signing sync push`,
`asc signing sync pull`, and, for object storage,
`asc signing sync rotate-password` commands. It adds no new command group and no second
encryption format. Encrypted git remains the default and keeps its current
invocation, flags, and output.

```sh
# Default, unchanged.
asc signing sync push --bundle-id com.example.app --profile-type IOS_APP_STORE \
  --repo git@github.com:team/certs.git --password-file ~/.config/asc/signing-sync-password

# Experimental GitLab Secure Files.
asc signing sync push --bundle-id com.example.app --profile-type IOS_APP_STORE \
  --storage gitlab-secure-files --gitlab-project 1234 --prefix asc-signing \
  --gitlab-token-file ~/.config/asc/gitlab-token \
  --password-file ~/.config/asc/signing-sync-password

# Experimental AWS Secrets Manager.
asc signing sync pull --storage aws-secrets-manager --region us-east-1 \
  --prefix asc-signing --password-file ~/.config/asc/signing-sync-password \
  --output-dir ./signing

# Experimental S3-compatible object storage.
asc signing sync push --bundle-id com.example.app --profile-type IOS_APP_STORE \
  --storage object --object-bucket team-certs --object-prefix asc/ \
  --password-file ~/.config/asc/signing-sync-password
```

## Storage selection

`--storage` accepts `git` (default), `gitlab-secure-files`,
`aws-secrets-manager`, and `object`. Any other value is a usage error; no unsupported value
is silently ignored.

| Storage | Required locators | Rejected locators |
| --- | --- | --- |
| `git` | `--repo` | `--prefix`, `--region`, `--gitlab-*`, `--object-*` |
| `gitlab-secure-files` | `--gitlab-project`, `--prefix`, `--gitlab-token-file` | `--repo`, `--branch`, `--region`, `--object-*` |
| `aws-secrets-manager` | `--prefix`, `--region` | `--repo`, `--branch`, `--gitlab-*`, `--object-*` |
| `object` | `--object-bucket` | `--repo`, `--branch`, `--prefix`, `--region`, `--gitlab-*` |

`--gitlab-host` is optional and defaults to `https://gitlab.com`. It must be an
https URL without embedded credentials. The GitLab token is read only through
the existing protected secret-file helper, is sent only in the `PRIVATE-TOKEN`
request header, and never appears in output, diagnostics, or errors. AWS
credentials come from the standard AWS environment that the SDK already
expects; this change adds no credential file format.

`--object-prefix`, `--object-region`, and `--object-endpoint` are optional.
Mixed backend flags are usage errors (exit code 2) raised before any secret
read or network request.

`asc signing sync rotate-password` supports `git` and `object`. Invoking it
with GitLab Secure Files or AWS Secrets Manager returns a usage error rather
than a partial rotation.

## Artifact model

Every backend transports already-encrypted bytes. Push still encrypts into an
isolated temporary working tree with the existing envelope, metadata,
repository path validation, artifact count limit, and cumulative size limit;
pull still decrypts and validates from that tree. The storage backend only
replaces the clone and publish steps:

- `git`: clone, commit, push, unchanged.
- `gitlab-secure-files`: download every secure file under the prefix, then
  upload or replace changed artifacts.
- `aws-secrets-manager`: read every prefixed secret, then create or update the
  corresponding secrets.
- `object`: read every `.enc` object under the prefix, then conditionally
  create or replace changed objects.

Each remote request is bounded by the shared CLI request timeout, and GitLab
uploads use the shared upload timeout, so one stalled request cannot hang a
multi-artifact push or pull while the batch itself stays uncapped. AWS
credential discovery shares the same budget because the default provider chain
can reach container or instance metadata over the network.

Artifact-count limits apply to the configured prefix, not to the whole remote
namespace, so a project or account holding unrelated files still syncs. GitLab
pagination is bounded separately by a page budget.

A stored object is named `<prefix>/<encrypted relative path>.enc`. The relative
path passes the existing encrypted-path validator, and remote names are
additionally rejected when they contain a traversal component, so a hostile
remote name cannot escape the working tree.

## GitLab Secure Files

The GitLab API v4 project-level secure files endpoints are used directly with
`net/http`; no GitLab SDK is added.

- `GET /api/v4/projects/:id/secure_files` lists files, paginated through
  `X-Next-Page`.
- `POST /api/v4/projects/:id/secure_files` uploads `name` and `file`. GitLab
  stores the record under the required `name` attribute, so the full scoped
  name including the prefix and relative path is preserved; the multipart part
  filename is not the stored name. The upload response is checked against the
  requested name, so an instance that stored the artifact elsewhere fails the
  push instead of leaving an artifact a later pull cannot find.
- `GET /api/v4/projects/:id/secure_files/:id/download` returns file content.
- `DELETE /api/v4/projects/:id/secure_files/:id` removes a file.

The create endpoint documents `name` and `file` as separate required
attributes, and the upstream implementation builds the record with
`secure_files.new(name: params[:name])` while assigning the uploaded part to
`secure_file.file`. The stored name therefore carries the prefix and relative
path, and the record has no format restriction beyond presence, per-project
uniqueness, and a path-traversal check that the existing encrypted-path
validator already satisfies.

Secure file names are unique per project and the API has no replace operation,
so an artifact whose content changed is deleted and re-uploaded. The previous
ciphertext is downloaded and checksum-verified first and is re-uploaded if the
replacement upload fails. Recovery uses its own bounded context so canceling
the command does not also cancel restoration. A failed restoration is reported;
GitLab does not provide an atomic replacement transaction. An artifact whose published sha256 checksum already matches the local
ciphertext is skipped, so unchanged pushes perform no deletion. Files outside
the configured prefix are never listed into scope, replaced, or deleted, and no
cleanup of unknown files is performed.

Failures are closed: a non-JSON or HTML response, a redirect to another host or
scheme, a checksum mismatch, an artifact above the existing encrypted size
limit, or an artifact above GitLab's documented 5 MiB secure file limit stops
the operation. Errors carry the HTTP status and GitLab's public error title
after control characters are removed, the title is truncated, and any token
occurrence is redacted.

## AWS Secrets Manager

One secret holds one encrypted artifact. The secret string is the ciphertext in
standard base64 so the SDK's string secret round-trips losslessly. `CreateSecret`
publishes a new artifact, and `PutSecretValue` updates an existing one. A create
collision fails with a refetch instruction: the service listing can lag a new
secret, so a collision must never overwrite ciphertext the invocation did not
fetch and validate. Before updating an existing secret, the current ciphertext
must match the last successful fetch or publication by this store instance;
a changed or newly visible secret fails with a refetch instruction. An existing
secret whose value already matches is skipped so repeated pushes do not consume
the account's secret version quota. No secret is deleted.

These checks detect changes observed before a write, but AWS `PutSecretValue`
has no compare-and-swap condition: another writer can still race between the
check and update. GitLab replacement likewise has no whole-store transaction.
Serialize writers to a shared prefix. A multi-artifact failure can leave a
partially published store; neither backend promises atomic publication.

AWS errors preserve recognized service error codes, HTTP status, cancellation,
and read-only refusals. Raw SDK and credential-process messages are omitted
because malformed credential-process output can contain credentials.

The global `--read-only` and `ASC_READ_ONLY` modes allow remote fetches and local
encryption/decryption but refuse GitLab uploads/deletes and AWS creates/updates
before the remote mutation request. AWS listing and retrieval remain permitted
even though the SDK transports those reads as HTTP POSTs.

An artifact whose base64 encoding exceeds 60 KiB fails before any AWS call and
explains the limit rather than truncating. Secret names are validated against
the documented AWS character set, so an artifact path that AWS cannot name
fails closed instead of being silently renamed.

## S3-compatible object storage

Object keys use the git working tree layout exactly:
`<object-prefix>/<encrypted relative path>.enc`, or the bare relative path when
no prefix is given. A bucket prefix and an encrypted git repository can
therefore be mirrored byte for byte. One trailing slash on `--object-prefix` is
accepted, and the prefix otherwise follows the shared prefix rules. The bucket
must be 3-63 lowercase letters, digits, dots, or hyphens, must not look like an
IPv4 address, and must not use a reserved S3 prefix (`xn--`, `sthree-`,
`amzn-s3-demo-`). Reserved suffixes are accepted because access point aliases
and directory bucket names are valid in object requests. `--object-endpoint` accepts an HTTPS origin
without credentials, path, query, or fragment and switches to path-style
addressing for S3-compatible services.

The implementation uses the AWS SDK S3 client that the repository already
depends on; no dependency is added. Credentials and, unless `--object-region`
is set, the region come from the standard AWS configuration chain:
environment variables, shared configuration or SSO profiles, web identity for
workload identity, and container or instance metadata. No flag accepts a
credential. A missing region fails before any request. `AWS_CA_BUNDLE` is
honored, and signed requests never follow redirects.

Fetch lists the prefix, downloads every `.enc` object, and records each
object's entity tag. Publish skips unchanged artifacts, creates new objects
with `If-None-Match: *`, and replaces existing objects with `If-Match` on the
fetched entity tag. A precondition failure is reported as a conflict that
tells the operator to pull and retry, with exit code 1; newer data is never
overwritten. Services that ignore conditional request headers cannot provide
this guarantee. Push never deletes objects, and a multi-artifact push that hits
a conflict can leave the artifacts before the conflict published.

Rotation cannot replace several objects atomically, so it writes new objects
first and then swaps them in:

1. Stage. Every re-encrypted artifact is written with `If-None-Match: *` to
   `<key>.asc-rotation-<random id>`. Staged keys do not end in `.enc`, so
   fetches ignore them. A failure deletes the staged objects and leaves every
   live artifact on the current password.
2. Verify. A `HEAD` of every live key must return the fetched entity tag. A
   concurrent push aborts the rotation before any live write, deletes the
   staged objects, and exits 1.
3. Swap. Each live object is replaced with `If-Match` on its fetched entity
   tag. If one replacement fails, the already replaced objects are restored
   from the fetched ciphertext, again conditionally, using a recovery context
   that survives command cancellation. After a complete rollback, every
   artifact still uses the current password and the staged objects are
   deleted.

Remaining failure modes:

- Rollback also fails. The error names the artifacts that now use the new
  password, and the staged objects are kept. Every staged object holds the new
  ciphertext, so copying each one over its live key finishes the rotation.
- The process dies during the swap. The store is mixed, and the staged objects
  still hold the complete new-password set, so the same recovery applies.
- The service returns no entity tag for a replaced object. The rotation stops
  and rolls back the earlier objects. It reports that object as using the new
  password instead of restoring it with an unconditional write.
- Staged cleanup fails after an abort or rollback. The error names each
  leftover staged key. Cleanup runs with a recovery context, so a canceled
  command still removes its staged objects.
- Staged cleanup fails after a successful swap. The command succeeds and warns
  on stderr about each leftover staged key. Leftover staged objects are
  ignored by pulls and safe to delete.

Read-only mode allows fetches and refuses every object `PUT` and `DELETE`
before the request. Errors keep S3 error codes and HTTP status while dropping
provider messages and credential-process output.

## Output and exit codes

Every existing field keeps its meaning. `repoUrl` carries the redacted git
remote for git storage and a non-secret locator for remote storage
(`gitlab-secure-files://host/projects/<id>/<prefix>`,
`aws-secrets-manager://<region>/<prefix>`, or `s3://<bucket>/<prefix>`). The
object storage locator omits any custom endpoint host.

Push, pull, and rotate-password receipts also include an additive `storage`
object for every backend. `kind` is the `--storage` value, and `location` is
the same locator as `repoUrl`. For git storage only, `branch` names the branch:

```json
{"operation":"pull","repoUrl":"s3://team-certs/asc","storage":{"kind":"object","location":"s3://team-certs/asc"},"bundleId":"","profileType":"","files":[],"identityPresent":false}
{"operation":"pull","repoUrl":"git@github.com:team/certs.git","storage":{"kind":"git","location":"git@github.com:team/certs.git","branch":"main"},"bundleId":"","profileType":"","files":[],"identityPresent":false}
```

The receipt has no version identifier. A push or rotation writes many objects,
secrets, or files, each with its own version, and the Git commit is not
reported today, so no single value would identify the stored state.

Data goes to stdout and progress to stderr. Invalid flag combinations use exit
code 2 and operational failures use exit code 1.

## Tests

RED-GREEN coverage includes an httptest GitLab server that round-trips
encrypted bytes through push and pull, asserts the token header, asserts that
errors keep the status and title without the token, and rejects HTML responses,
cross-host redirects, oversize downloads, oversize uploads, out-of-prefix
names, and traversal names. AWS coverage uses a stub client so no test dials
AWS and asserts create/put names, base64 round-trips, that no plaintext is
stored, and that an oversize artifact fails before the first AWS call. Command
coverage asserts the storage selection matrix, the experimental help text, the
protected token-file contract, and that rotation rejects GitLab and AWS
Secrets Manager storage. Object storage coverage uses an in-process,
TLS-served, path-style S3 fake driven through the real SDK. It covers
byte-identical git-layout round trips, conditional creates and replacements,
concurrent-writer conflicts on push and rotation, rotation staging, swap,
rollback, and mixed-state reporting, read-only refusals, pagination, unsafe
keys, size and count limits, locator validation, and command-level pull and
rotation. No test contacts a real bucket. Existing git
signing sync tests are unchanged and still pass.

## Alternatives

Adding a second encryption path per backend was rejected: the backends
transport ciphertext only, so encryption, metadata authentication, and path
validation stay in one place. Deleting remote artifacts that no longer exist
locally was rejected because a shared store may hold artifacts written by other
teams or tools. Implementing rotation for GitLab Secure Files and AWS Secrets
Manager was rejected for now because neither backend offers conditional
writes, so a concurrent push could be overwritten. Object storage rotation
relies on conditional writes plus staging and rollback instead of a whole-store
transaction. A pointer object that switches between complete generations was
rejected because it would break the byte-for-byte git layout.
