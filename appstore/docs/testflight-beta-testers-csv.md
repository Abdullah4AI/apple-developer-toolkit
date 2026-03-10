# TestFlight Beta Testers CSV Standard

This document defines the CSV contract for:

- `asc testflight testers export`
- `asc testflight testers import`

## Canonical Schema

Header row (required for canonical mode):

```text
email,first_name,last_name,groups
```

- `email` (required): tester email address
- `first_name` (optional): tester first name
- `last_name` (optional): tester last name
- `groups` (optional): one or more group names/IDs

### Group Delimiter

- Export uses **semicolon** delimiter in `groups`:
  - `Alpha;Beta`
- Import accepts **semicolon** delimiters in canonical mode:
  - `Alpha;Beta`
- For compatibility, import also accepts comma delimiters **when no semicolon is present**:
  - `Alpha,Beta`

## Compatibility Modes (Import)

Import supports these additional formats for compatibility with fastlane/pilot:

1. Header aliases:

```text
First,Last,Email,Groups
```

2. Legacy headerless rows:

```text
First,Last,Email[,Groups]
```

## Validation Rules

- Unknown header columns are rejected.
- Duplicate header columns are rejected.
- Empty header names are rejected.
- Duplicate emails in input are rejected (row-level failure in import summary).
- Invalid email format is rejected (row-level failure in import summary).
- Headerless legacy rows must have 3 or 4 columns.

## Determinism

- Export rows are sorted by email (case-insensitive).
- With `--include-groups`, group lists are stable and semicolon-delimited.

## Test Fixture

A random-name fixture for parser/import testing is included at:

`internal/cli/testflight/testdata/beta_testers_random.csv`
