# Official search optimization plan

Release 4.4.4 adds a read-only workflow that turns official
Apple Ads Platform API v1 evidence and App Store Connect metadata into one
reviewable search plan:

```bash
asc optimize search plan \
  --app "APP_ID" \
  --version "APP_VERSION" \
  --ad-account "AD_ACCOUNT_ID" \
  --country "US" \
  --genre "PRODUCTIVITY_UTILITIES" \
  --locale "en-US" \
  --out-dir ".asc/optimization/APP_VERSION" \
  --output markdown
```

The command keeps Apple's metrics semantically separate: Apple's 1–5 and
1–100 popularity scores are market search demand, impression share is
app-specific paid reach, search-term report rows are actual paid outcomes, and
metadata coverage comes from the selected App Store localization. Deterministic
actions identify converting broad terms,
costly zero-install terms, metadata candidates, low-share opportunities, and
already-saturated terms without inventing organic rank or proprietary
difficulty scores.

When `--out-dir` is set, the command writes a canonical report plus review-only
metadata, exact-keyword, and negative-keyword import files. It never applies
those files or mutates campaigns or App Store metadata. Partial Apple Ads
source failures remain visible in the report, while successful official data
is preserved. Actions that require proving a keyword is absent are suppressed
when their dependency is unavailable, and table or Markdown output shows the
source matrix and notices directly.

For apps with multiple App Info records, the command matches App Info to the
selected version state. If that match is ambiguous, pass the explicit record
with `--app-info`. The JSON report records both the resolved version ID and App
Info ID so the evidence can be reproduced.

The report also preserves Apple's app-and-country target-CPA suggestion for a
new Maximize Conversions campaign separately from recommendations for existing
campaigns.

App Store Connect and Apple Ads credentials are independent. Run `asc auth`
for App Store Connect and `asc ads auth login` for Apple Ads before using the
workflow. Credentialed live verification is still recommended before relying
on generated candidates in production.
