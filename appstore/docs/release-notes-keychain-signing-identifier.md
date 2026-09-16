# Stable code-signing identifier for keychain access

Release binaries for macOS are now signed with the fixed identifier
`com.rorkai.asc`. Earlier releases let `codesign` derive the identifier from
the versioned file name (for example `asc_5.3.0_macOS_arm64`), so every
release carried a different designated requirement. macOS keychain access
lists trust applications by that requirement, which is why "Always Allow" on
the stored `ASC API Key` item did not carry over to the next upgrade.

Resolving one profile also reads only that profile's keychain secret now.
Previously every stored profile was read to select one, so each stored
profile raised its own authorization prompt on every command.

The first run of a release with the new identifier prompts once more per
stored profile because the identifier changed. Choose "Always Allow" and
later upgrades reuse that decision.

Fixes #2512.
