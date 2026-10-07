package xcode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInferSigningPlanUsesEffectiveSuppliedOverrideAndReplays(t *testing.T) {
	requireStrictSigningPlatform(t)
	for _, selector := range []string{"PROVISIONING_PROFILE_SPECIFIER", "PROVISIONING_PROFILE"} {
		t.Run(selector, func(t *testing.T) {
			project := writeInferredSigningProject(t)
			root := t.TempDir()
			dev := writeSigningTestProfile(t, filepath.Join(root, "Dev.mobileprovision"), "Development", "11111111-1111-1111-1111-111111111111", "ABCDE12345.com.example.demo", time.Now().Add(48*time.Hour))
			store := writeSigningTestProfileWith(t, filepath.Join(root, "Store.mobileprovision"), "Store", "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA", "ABCDE12345.com.example.demo", time.Now().Add(time.Hour), func(p map[string]any) { delete(p, "ProvisionedDevices") })
			value := "Store"
			if selector == "PROVISIONING_PROFILE" {
				value = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
			}
			settings := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settings, `{"schemaVersion":1,"targets":[{"name":"App","configurations":[{"name":"Release","settings":{"`+selector+`":"`+value+`"}}]}]}`)
			plan, err := BuildSigningPlan(SigningPlanOptions{ProjectPath: project, SettingsFilePath: settings, ProfilePaths: []string{dev, store}, Configuration: "Release", SkipTargets: []string{"Widget", "Watch"}, StateDir: filepath.Join(root, "state")})
			if err != nil {
				t.Fatal(err)
			}
			if !plan.Ready || plan.ExportOptions == nil || plan.ExportOptions.Method != "app-store" || plan.ExportOptions.ProvisioningProfiles["com.example.demo"] != value {
				t.Fatalf("ready=%t blockers=%v export=%#v", plan.Ready, plan.Blockers, plan.ExportOptions)
			}
			if len(plan.Inferences) != 1 || plan.Inferences[0].ProfileUUID != "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA" || plan.Inferences[0].ProfilePath != store {
				t.Fatalf("wrong effective provenance: %#v", plan.Inferences)
			}
			if selector == "PROVISIONING_PROFILE" && signingPlanSettingEquals(plan, "App", "Release", "PROVISIONING_PROFILE_SPECIFIER", "Store") {
				t.Fatal("legacy UUID override retained inferred specifier")
			}
			if err := WriteSigningPlanArtifact(plan, false); err != nil {
				t.Fatal(err)
			}
			if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath}); err != nil {
				t.Fatalf("unchanged effective override could not apply: %v", err)
			}
			projectBytes, err := os.ReadFile(filepath.Join(project, "project.pbxproj"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(projectBytes), value) {
				t.Fatalf("applied project omitted effective profile %s", value)
			}
		})
	}
}

func TestInferSigningPlanBlocksIneligibleSuppliedOverride(t *testing.T) {
	requireStrictSigningPlatform(t)
	cases := []struct {
		name, settings, method string
		mutate                 func(map[string]any)
	}{
		{name: "bundle override", settings: `"PRODUCT_BUNDLE_IDENTIFIER":"com.example.other"`},
		{name: "known profile bundle", settings: `"PROVISIONING_PROFILE_SPECIFIER":"Known"`, mutate: func(p map[string]any) {
			p["Entitlements"].(map[string]any)["application-identifier"] = "ABCDE12345.com.example.other"
		}},
		{name: "known profile team", settings: `"PROVISIONING_PROFILE_SPECIFIER":"Known","DEVELOPMENT_TEAM":"ZYXWV98765"`},
		{name: "known profile method", settings: `"PROVISIONING_PROFILE_SPECIFIER":"Known"`, method: "app-store"},
		{name: "known expired", settings: `"PROVISIONING_PROFILE_SPECIFIER":"Known"`, mutate: func(p map[string]any) { p["ExpirationDate"] = time.Now().Add(-time.Hour) }},
		{name: "known platform", settings: `"PROVISIONING_PROFILE_SPECIFIER":"Known"`, mutate: func(p map[string]any) { p["Platform"] = []string{"OSX"} }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			project := writeInferredSigningProject(t)
			// Give the App a concrete iOS SDK so the incompatible OSX profile is filtered.
			pbx := filepath.Join(project, "project.pbxproj")
			data, err := os.ReadFile(pbx)
			if err != nil {
				t.Fatal(err)
			}
			data = []byte(strings.ReplaceAll(string(data), "PRODUCT_BUNDLE_IDENTIFIER = com.example.demo;", "PRODUCT_BUNDLE_IDENTIFIER = com.example.demo; SDKROOT = iphoneos;"))
			if err := os.WriteFile(pbx, data, 0o600); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			profile := writeSigningTestProfileWith(t, filepath.Join(root, "Known.mobileprovision"), "Known", "known-uuid", "ABCDE12345.com.example.demo", time.Now().Add(time.Hour), tt.mutate)
			settings := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settings, `{"schemaVersion":1,"targets":[{"name":"App","configurations":[{"name":"Release","settings":{`+tt.settings+`}}]}]}`)
			plan, err := BuildSigningPlan(SigningPlanOptions{ProjectPath: project, SettingsFilePath: settings, ProfilePaths: []string{profile}, ExportMethod: tt.method, Configuration: "Release", SkipTargets: []string{"Widget", "Watch"}, StateDir: filepath.Join(root, "state")})
			if err != nil {
				t.Fatal(err)
			}
			if plan.Ready || len(plan.Blockers) == 0 {
				t.Fatalf("ineligible supplied profile accepted: ready=%t blockers=%v export=%#v", plan.Ready, plan.Blockers, plan.ExportOptions)
			}
		})
	}
}

func TestInferSigningPlanBlocksConflictingConfigurationProfiles(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeInferredSigningProject(t)
	root := t.TempDir()
	a := writeSigningTestProfile(t, filepath.Join(root, "A.mobileprovision"), "A", "a-uuid", "ABCDE12345.com.example.demo", time.Now().Add(48*time.Hour))
	b := writeSigningTestProfile(t, filepath.Join(root, "B.mobileprovision"), "B", "b-uuid", "ABCDE12345.com.example.demo", time.Now().Add(time.Hour))
	settings := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settings, `{"schemaVersion":1,"targets":[{"name":"App","configurations":[{"name":"Debug","settings":{"PROVISIONING_PROFILE_SPECIFIER":"A"}},{"name":"Release","settings":{"PROVISIONING_PROFILE_SPECIFIER":"B"}}]}]}`)
	opts := SigningPlanOptions{ProjectPath: project, SettingsFilePath: settings, ProfilePaths: []string{a, b}, SkipTargets: []string{"Widget", "Watch"}, StateDir: filepath.Join(root, "state")}
	plan, err := BuildSigningPlan(opts)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Ready || plan.ExportOptions != nil || !strings.Contains(strings.Join(plan.Blockers, " "), "--configuration") {
		t.Fatalf("configuration collision accepted: ready=%t blockers=%v export=%#v", plan.Ready, plan.Blockers, plan.ExportOptions)
	}
	opts.Configuration = "Debug"
	plan, err = BuildSigningPlan(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Ready || plan.ExportOptions == nil || plan.ExportOptions.ProvisioningProfiles["com.example.demo"] != "A" {
		t.Fatalf("explicit Debug selection failed: ready=%t blockers=%v export=%#v", plan.Ready, plan.Blockers, plan.ExportOptions)
	}
}

func TestInferSigningPlanAcceptsConfigurationProfileAliases(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeInferredSigningProject(t)
	root := t.TempDir()
	uuid := "11111111-1111-1111-1111-111111111111"
	profile := writeSigningTestProfile(t, filepath.Join(root, "A.mobileprovision"), "A", uuid, "ABCDE12345.com.example.demo", time.Now().Add(time.Hour))
	settings := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settings, `{"schemaVersion":1,"targets":[{"name":"App","configurations":[{"name":"Debug","settings":{"PROVISIONING_PROFILE_SPECIFIER":"A"}},{"name":"Release","settings":{"PROVISIONING_PROFILE":"`+uuid+`"}}]}]}`)
	plan, err := BuildSigningPlan(SigningPlanOptions{ProjectPath: project, SettingsFilePath: settings, ProfilePaths: []string{profile}, SkipTargets: []string{"Widget", "Watch"}, StateDir: filepath.Join(root, "state")})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Ready || plan.ExportOptions == nil {
		t.Fatalf("same profile name/UUID rejected: ready=%t blockers=%v", plan.Ready, plan.Blockers)
	}
	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath}); err != nil {
		t.Fatalf("alias plan could not replay: %v", err)
	}
}

func TestInferSigningPlanLegacyRemovalKeepsInferredSpecifier(t *testing.T) {
	requireStrictSigningPlatform(t)
	for _, removal := range []string{`null`} {
		t.Run(removal, func(t *testing.T) {
			project := writeInferredSigningProject(t)
			root := t.TempDir()
			profile := writeSigningTestProfile(t, filepath.Join(root, "A.mobileprovision"), "A", "11111111-1111-1111-1111-111111111111", "ABCDE12345.com.example.demo", time.Now().Add(time.Hour))
			settings := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settings, `{"schemaVersion":1,"targets":[{"name":"App","configurations":[{"name":"Release","settings":{"PROVISIONING_PROFILE":`+removal+`}}]}]}`)
			plan, err := BuildSigningPlan(SigningPlanOptions{ProjectPath: project, SettingsFilePath: settings, ProfilePaths: []string{profile}, Configuration: "Release", SkipTargets: []string{"Widget", "Watch"}, StateDir: filepath.Join(root, "state")})
			if err != nil {
				t.Fatal(err)
			}
			if !plan.Ready || !signingPlanSettingEquals(plan, "App", "Release", "PROVISIONING_PROFILE_SPECIFIER", "A") || plan.ExportOptions == nil || plan.ExportOptions.ProvisioningProfiles["com.example.demo"] != "A" {
				t.Fatalf("legacy removal discarded inferred profile: ready=%t blockers=%v export=%#v", plan.Ready, plan.Blockers, plan.ExportOptions)
			}
		})
	}
}

func TestInferSigningPlanSelectsSuppliedTeamWithoutProfileSelector(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeInferredSigningProject(t)
	root := t.TempDir()
	a := writeSigningTestProfile(t, filepath.Join(root, "A.mobileprovision"), "A", "11111111-1111-1111-1111-111111111111", "ABCDE12345.com.example.demo", time.Now().Add(48*time.Hour))
	b := writeSigningTestProfileWith(t, filepath.Join(root, "B.mobileprovision"), "B", "22222222-2222-2222-2222-222222222222", "ZYXWV98765.com.example.demo", time.Now().Add(time.Hour), func(p map[string]any) {
		p["TeamIdentifier"] = []string{"ZYXWV98765"}
		p["ApplicationIdentifierPrefix"] = []string{"ZYXWV98765"}
		p["Entitlements"].(map[string]any)["com.apple.developer.team-identifier"] = "ZYXWV98765"
	})
	settings := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settings, `{"schemaVersion":1,"targets":[{"name":"App","configurations":[{"name":"Release","settings":{"DEVELOPMENT_TEAM":"ZYXWV98765"}}]}]}`)
	plan, err := BuildSigningPlan(SigningPlanOptions{ProjectPath: project, SettingsFilePath: settings, ProfilePaths: []string{a, b}, Configuration: "Release", SkipTargets: []string{"Widget", "Watch"}, StateDir: filepath.Join(root, "state")})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Ready || len(plan.Inferences) != 1 || plan.Inferences[0].ProfilePath != b || plan.ExportOptions == nil || plan.ExportOptions.ProvisioningProfiles["com.example.demo"] != "B" {
		t.Fatalf("team-selected inference wrong: ready=%t blockers=%v inferences=%#v export=%#v", plan.Ready, plan.Blockers, plan.Inferences, plan.ExportOptions)
	}
	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath}); err != nil {
		t.Fatalf("team-only override could not replay: %v", err)
	}
}

func TestInferSigningPlanRejectsInvalidProfileDates(t *testing.T) {
	requireStrictSigningPlatform(t)
	cases := []struct {
		name, want string
		mutate     func(map[string]any)
	}{
		{name: "missing expiry", want: "missing expiration date", mutate: func(p map[string]any) { delete(p, "ExpirationDate") }},
		{name: "zero expiry", want: "expiration date", mutate: func(p map[string]any) { p["ExpirationDate"] = time.Time{} }},
		{name: "future creation", want: "creation date is in the future", mutate: func(p map[string]any) { p["CreationDate"] = time.Now().Add(24 * time.Hour) }},
		{name: "invalid creation", want: "invalid creation date", mutate: func(p map[string]any) { p["CreationDate"] = "not-a-date" }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			project := writeInferredSigningProject(t)
			root := t.TempDir()
			profile := writeSigningTestProfileWith(t, filepath.Join(root, "A.mobileprovision"), "A", "11111111-1111-1111-1111-111111111111", "ABCDE12345.com.example.demo", time.Now().Add(time.Hour), tt.mutate)
			_, err := BuildSigningPlan(SigningPlanOptions{ProjectPath: project, ProfilePaths: []string{profile}, Configuration: "Release", SkipTargets: []string{"Widget", "Watch"}, StateDir: filepath.Join(root, "state")})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v, want %s", err, tt.want)
			}
		})
	}
}
