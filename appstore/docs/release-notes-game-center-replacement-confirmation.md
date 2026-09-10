# Game Center relationship replacement confirmation

> Status: the compatibility window described below closed in 5.0.0. The
> relationship setters now reject invocations without `--confirm` before any
> request. See `migrate-to-5-0.mdx`.

Game Center relationship setters now accept an `--confirm` flag.
Invocations that omit it continue to work in the 4.x compatibility window and
emit a deprecation warning. Starting in 5.0.0, `--confirm` will be required to
acknowledge that these setters replace the complete relationship collection and
may remove existing members.
