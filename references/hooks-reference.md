# Apple Developer Toolkit - Hooks Reference

Complete catalog of all 42 hook events with their context variables.

## Configuration

- **Global config:** `~/.appledev/hooks.yaml`
- **Project config:** `.appledev/hooks.yaml` (merges with global)
- **Hook scripts:** `~/.appledev/hooks/`
- **Logs:** `~/.appledev/hook-logs/`

## Hook Definition Schema

```yaml
hooks:
  event.name:
    - name: "hook-name"          # Identifier
      notify: telegram           # Notification target (optional)
      template: "Message text"   # Notification template (optional)
      run: "command to execute"  # Shell command (optional)
      when: always               # always | success | failure
```

### Template Variables

Use `{{.VAR_NAME}}` syntax. All context variables listed below are available in templates and run commands.

Built-in variables (always available):
- `{{.EVENT}}` - Event name
- `{{.TIMESTAMP}}` - ISO timestamp
- `{{.STATUS}}` - success/failure/unknown

---

## Build Events (13 events)

Events fired during `appledev build` operations.

| Event | Fires When | Context Variables |
|-------|-----------|-------------------|
| `build.start` | Build begins | `APP_NAME`, `PLATFORM`, `MODEL` |
| `build.analyze.done` | Analysis phase complete | `APP_NAME`, `FEATURES_COUNT` |
| `build.plan.done` | Build plan generated | `APP_NAME`, `FILES_COUNT` |
| `build.generate.done` | Code generation complete | `APP_NAME`, `FILES_GENERATED` |
| `build.compile.start` | Xcode compilation begins | `APP_NAME`, `PROJECT_PATH` |
| `build.compile.success` | Compilation succeeds | `APP_NAME`, `BUILD_TIME_SEC` |
| `build.compile.failure` | Compilation fails | `APP_NAME`, `ERROR_COUNT`, `ERRORS` |
| `build.fix.start` | Auto-fix begins | `APP_NAME`, `ERROR_COUNT` |
| `build.fix.done` | Auto-fix complete | `APP_NAME`, `FIXES_APPLIED`, `REMAINING_ERRORS` |
| `build.run.start` | Simulator launch begins | `APP_NAME`, `SIMULATOR_ID` |
| `build.run.success` | App running in simulator | `APP_NAME`, `SIMULATOR_ID` |
| `build.chat.message` | User sends chat message | `APP_NAME`, `MESSAGE` |
| `build.done` | Full build pipeline complete | `APP_NAME`, `STATUS`, `DURATION_SEC` |

---

## Store Events (20 events)

Events fired during `appledev store` operations.

| Event | Fires When | Context Variables |
|-------|-----------|-------------------|
| `store.auth.login` | Authentication succeeds | `PROFILE_NAME`, `KEY_ID` |
| `store.auth.failure` | Authentication fails | `PROFILE_NAME`, `ERROR` |
| `store.upload.start` | Build upload begins | `APP_ID`, `IPA_PATH`, `BUILD_NUMBER` |
| `store.upload.progress` | Upload progress update | `APP_ID`, `PERCENT` |
| `store.upload.done` | Upload complete | `APP_ID`, `BUILD_ID`, `BUILD_NUMBER` |
| `store.upload.failure` | Upload fails | `APP_ID`, `ERROR` |
| `store.processing.start` | Build processing begins | `APP_ID`, `BUILD_ID` |
| `store.processing.done` | Build processed and ready | `APP_ID`, `BUILD_ID`, `PROCESSING_TIME_SEC` |
| `store.testflight.distribute` | Build distributed to TestFlight | `APP_ID`, `BUILD_ID`, `GROUP_NAME` |
| `store.submit.start` | App Store submission begins | `APP_ID`, `VERSION` |
| `store.submit.done` | Submission complete | `APP_ID`, `VERSION`, `SUBMISSION_ID` |
| `store.submit.failure` | Submission fails | `APP_ID`, `VERSION`, `ERRORS` |
| `store.review.status` | Review status changes | `APP_ID`, `VERSION`, `STATUS`, `PREVIOUS_STATUS` |
| `store.review.approved` | App approved | `APP_ID`, `VERSION` |
| `store.review.rejected` | App rejected | `APP_ID`, `VERSION`, `REJECTION_REASONS` |
| `store.release.done` | App released to App Store | `APP_ID`, `VERSION`, `RELEASE_DATE` |
| `store.validate.pass` | Pre-submission validation passes | `APP_ID`, `VERSION` |
| `store.validate.fail` | Pre-submission validation fails | `APP_ID`, `VERSION`, `ISSUES` |
| `store.crash.new` | New crash report from TestFlight | `APP_ID`, `BUILD_ID`, `CRASH_SUMMARY` |
| `store.feedback.new` | New TestFlight feedback | `APP_ID`, `BUILD_ID`, `FEEDBACK` |

---

## Docs Events (4 events)

Events fired during documentation search operations (cli.js).

| Event | Fires When | Context Variables |
|-------|-----------|-------------------|
| `docs.search` | Documentation search executed | `QUERY`, `RESULTS_COUNT` |
| `docs.wwdc.search` | WWDC search executed | `QUERY`, `YEAR`, `RESULTS_COUNT` |
| `docs.cache.miss` | Local cache miss | `QUERY` |
| `docs.index.rebuild` | WWDC index rebuilt | `VIDEOS_COUNT`, `YEARS_COUNT` |

---

## Pipeline Events (5 events)

Cross-tool pipeline events for chained workflows.

| Event | Fires When | Context Variables |
|-------|-----------|-------------------|
| `pipeline.start` | Named pipeline begins | `PIPELINE_NAME`, `STEPS_COUNT` |
| `pipeline.step.done` | Pipeline step completes | `PIPELINE_NAME`, `STEP_NAME`, `STATUS` |
| `pipeline.done` | Full pipeline completes | `PIPELINE_NAME`, `STATUS`, `DURATION_SEC` |
| `pipeline.failure` | Pipeline fails at any step | `PIPELINE_NAME`, `FAILED_STEP`, `ERROR` |

---

## When Conditions

| Value | Behavior |
|-------|----------|
| `always` | Hook runs regardless of STATUS |
| `success` | Hook runs only when STATUS=success |
| `failure` | Hook runs only when STATUS=failure |

## Config Merge Strategy

When both global (`~/.appledev/hooks.yaml`) and project (`.appledev/hooks.yaml`) configs exist:

- **Same event, different hook names:** Both hooks run (project first, then global)
- **Same event, same hook name:** Project hook overrides global
- **Notifiers:** Project inherits global notifier config

## Built-in Hook Scripts

Located in `~/.appledev/hooks/` after running `hook-init.sh`:

| Script | Purpose |
|--------|---------|
| `notify-telegram.sh` | Send Telegram notification |
| `git-tag-release.sh` | Create and push git tag |
| `run-swift-tests.sh` | Run Swift tests (SPM or Xcode) |
| `generate-changelog.sh` | Generate changelog from git history |

## Examples

### Notify on build completion
```yaml
hooks:
  build.done:
    - name: "notify"
      notify: telegram
      template: "Build {{.STATUS}} - {{.APP_NAME}} ({{.DURATION_SEC}}s)"
      when: always
```

### Auto-tag on approval
```yaml
hooks:
  store.review.approved:
    - name: "tag"
      run: "~/.appledev/hooks/git-tag-release.sh {{.VERSION}}"
      when: success
```

### Log errors to file
```yaml
hooks:
  build.compile.failure:
    - name: "log"
      run: "echo '{{.TIMESTAMP}} {{.APP_NAME}}: {{.ERRORS}}' >> ~/.appledev/hook-logs/errors.log"
      when: always
```

### Run tests after successful compile
```yaml
hooks:
  build.compile.success:
    - name: "tests"
      run: "~/.appledev/hooks/run-swift-tests.sh"
      when: success
```
