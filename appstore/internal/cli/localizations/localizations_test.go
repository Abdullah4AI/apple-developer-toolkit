package localizations

import (
	"reflect"
	"strings"
	"testing"
)

func TestLocalizationsCreateCommand_HelpMentionsCanonicalLocaleForms(t *testing.T) {
	cmd := LocalizationsCreateCommand()

	localeFlag := cmd.FlagSet.Lookup("locale")
	if localeFlag == nil {
		t.Fatal("expected --locale flag")
	}
	for _, want := range []string{"canonical ASC values", "ar-SA", "zh-Hans"} {
		if !strings.Contains(localeFlag.Usage, want) {
			t.Fatalf("expected --locale usage to contain %q, got %q", want, localeFlag.Usage)
		}
	}
	for _, want := range []string{
		`asc localizations supported-locales --version "VERSION_ID"`,
		`"ar" is usually rejected; use "ar-SA"`,
		`"de" should usually be "de-DE"`,
		`"zh-Hans-CN"`,
		`"zh-Hant-TW"`,
	} {
		if !strings.Contains(cmd.LongHelp, want) {
			t.Fatalf("expected long help to contain %q, got %q", want, cmd.LongHelp)
		}
	}
}

func TestLocalizationsUpdateCommand_HelpMentionsCanonicalLocaleForms(t *testing.T) {
	cmd := LocalizationsUpdateCommand()

	localeFlag := cmd.FlagSet.Lookup("locale")
	if localeFlag == nil {
		t.Fatal("expected --locale flag")
	}
	for _, want := range []string{"reuse exact ASC locale", "ar-SA", "zh-Hans"} {
		if !strings.Contains(localeFlag.Usage, want) {
			t.Fatalf("expected --locale usage to contain %q, got %q", want, localeFlag.Usage)
		}
	}
	for _, want := range []string{
		`asc localizations supported-locales --version "VERSION_ID"`,
		`asc localizations list --version "VERSION_ID"`,
		`asc localizations list --app "APP_ID" --type app-info`,
		`"ar" is usually stored as "ar-SA"`,
		`"de" is usually stored as "de-DE"`,
		`"zh-Hans-CN" and "zh-Hant-TW"`,
	} {
		if !strings.Contains(cmd.LongHelp, want) {
			t.Fatalf("expected long help to contain %q, got %q", want, cmd.LongHelp)
		}
	}
}

func TestLocalizationsListCommand_IncludeFlagListsSupportedValues(t *testing.T) {
	cmd := LocalizationsListCommand()

	includeFlag := cmd.FlagSet.Lookup("include")
	if includeFlag == nil {
		t.Fatal("expected --include flag on localizations list")
	}
	for _, want := range []string{"appStoreVersion", "appScreenshotSets", "appPreviewSets", "searchKeywords"} {
		if !strings.Contains(includeFlag.Usage, want) {
			t.Fatalf("expected --include usage to mention %q, got %q", want, includeFlag.Usage)
		}
	}
	if !strings.Contains(cmd.LongHelp, `asc localizations list --version "VERSION_ID" --include "appScreenshotSets,appPreviewSets"`) {
		t.Fatalf("expected long help to document an --include example, got %q", cmd.LongHelp)
	}
}

func TestAppInfoAttemptedFieldsKeepsWhitespaceOnlyValues(t *testing.T) {
	got := appInfoAttemptedFields(updateAppInfoParams{
		name:              " ",
		privacyChoicesURL: "\t",
	})

	want := []string{"name", "privacyChoicesUrl"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("appInfoAttemptedFields() = %v, want %v", got, want)
	}
}

func TestVersionAttemptedFieldsKeepsWhitespaceOnlyValues(t *testing.T) {
	got := versionAttemptedFields(updateVersionParams{
		description: " ",
		supportURL:  "\n",
	})

	want := []string{"description", "supportUrl"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("versionAttemptedFields() = %v, want %v", got, want)
	}
}
