package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/basecamp/cli/credstore"
	"github.com/basecamp/cli/output"
	"github.com/basecamp/cli/profile"
	"github.com/basecamp/fizzy-cli/internal/client"
	"github.com/basecamp/fizzy-cli/internal/config"
)

func TestDoctorResultSummary(t *testing.T) {
	tests := []struct {
		name   string
		result DoctorResult
		want   string
	}{
		{name: "all passed", result: DoctorResult{Passed: 4}, want: "All 4 checks passed"},
		{name: "all passed with skipped", result: DoctorResult{Passed: 4, Skipped: 1}, want: "All 4 checks passed, 1 skipped"},
		{name: "mixed", result: DoctorResult{Passed: 3, Failed: 1, Warned: 2, Skipped: 1}, want: "3 passed, 1 failed, 2 warnings, 1 skipped"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.Summary(); got != tt.want {
				t.Fatalf("Summary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDoctorCommandNoCredentials(t *testing.T) {
	configDir := t.TempDir()
	config.SetTestConfigDir(configDir)
	defer config.ResetTestConfigDir()

	mock := NewMockClient()
	result := SetTestModeWithSDK(mock)
	SetTestConfig("", "", testHTTPServer.URL)
	defer resetTest()

	cmd, _, err := rootCmd.Find([]string{"doctor"})
	if err != nil {
		t.Fatalf("find doctor command: %v", err)
	}

	err = cmd.RunE(cmd, []string{})
	assertExitCode(t, err, 0)

	if result.Response == nil || !result.Response.OK {
		t.Fatalf("expected OK response, got %#v", result.Response)
	}

	data, ok := result.Response.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map response data, got %#v", result.Response.Data)
	}
	checks, ok := data["checks"].([]any)
	if !ok {
		t.Fatalf("expected checks array, got %#v", data["checks"])
	}

	var sawCredentialsFail bool
	for _, item := range checks {
		check := item.(map[string]any)
		if check["name"] == "Credentials" {
			sawCredentialsFail = true
			if check["status"] != "fail" {
				t.Fatalf("expected credentials fail, got %#v", check)
			}
			if !strings.Contains(check["message"].(string), "No credentials found") {
				t.Fatalf("expected missing credentials message, got %#v", check)
			}
		}
	}
	if !sawCredentialsFail {
		t.Fatal("expected Credentials check in doctor output")
	}
}

func TestDoctorCommandHealthySetup(t *testing.T) {
	configDir := t.TempDir()
	config.SetTestConfigDir(configDir)
	defer config.ResetTestConfigDir()

	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{StatusCode: 200, Data: map[string]any{}}
	mock.OnGet("/my/identity.json", &client.APIResponse{StatusCode: 200, Data: map[string]any{
		"id":            "user-123",
		"email_address": "doctor@example.com",
		"accounts": []any{
			map[string]any{"id": "1", "slug": "/acme", "name": "Acme"},
		},
	}})
	mock.OnGet("/boards.json", &client.APIResponse{StatusCode: 200, Data: []any{
		map[string]any{"id": "board-1", "name": "Roadmap"},
	}})
	mock.OnGet("/boards/board-1.json", &client.APIResponse{StatusCode: 200, Data: map[string]any{
		"id":   "board-1",
		"name": "Roadmap",
	}})

	result := SetTestModeWithSDK(mock)
	SetTestConfig("test-token", "acme", testHTTPServer.URL)
	cfg.Board = "board-1"
	t.Setenv("FIZZY_TOKEN", "test-token")
	defer resetTest()

	cmd, _, err := rootCmd.Find([]string{"doctor"})
	if err != nil {
		t.Fatalf("find doctor command: %v", err)
	}

	err = cmd.RunE(cmd, []string{})
	assertExitCode(t, err, 0)

	if result.Response == nil || !result.Response.OK {
		t.Fatalf("expected OK response, got %#v", result.Response)
	}

	data := result.Response.Data.(map[string]any)
	checks := data["checks"].([]any)
	statuses := map[string]string{}
	for _, item := range checks {
		check := item.(map[string]any)
		statuses[check["name"].(string)] = check["status"].(string)
	}

	for _, name := range []string{"Credentials", "Authentication", "Account Access", "Default Board"} {
		if got := statuses[name]; got != "pass" {
			t.Fatalf("expected %s to pass, got %q", name, got)
		}
	}
}

func TestDoctorDetectsInvalidLocalConfig(t *testing.T) {
	workDir := t.TempDir()
	configDir := t.TempDir()
	config.SetTestWorkingDir(workDir)
	config.SetTestConfigDir(configDir)
	defer config.ResetTestWorkingDir()
	defer config.ResetTestConfigDir()

	if err := os.WriteFile(filepath.Join(workDir, config.LocalConfigFile), []byte("token: [broken"), 0o600); err != nil {
		t.Fatalf("write local config: %v", err)
	}

	mock := NewMockClient()
	result := SetTestModeWithSDK(mock)
	SetTestConfig("", "", testHTTPServer.URL)
	defer resetTest()

	cmd, _, err := rootCmd.Find([]string{"doctor"})
	if err != nil {
		t.Fatalf("find doctor command: %v", err)
	}

	err = cmd.RunE(cmd, []string{})
	assertExitCode(t, err, 0)

	data := result.Response.Data.(map[string]any)
	checks := data["checks"].([]any)
	for _, item := range checks {
		check := item.(map[string]any)
		if check["name"] == "Local Config" {
			if check["status"] != "fail" {
				t.Fatalf("expected Local Config fail, got %#v", check)
			}
			if !strings.Contains(check["message"].(string), "Invalid YAML") {
				t.Fatalf("expected invalid yaml message, got %#v", check)
			}
			return
		}
	}
	t.Fatal("expected Local Config check in output")
}

func TestDoctorWarnsOnLocalTokenStorage(t *testing.T) {
	workDir := t.TempDir()
	configDir := t.TempDir()
	config.SetTestWorkingDir(workDir)
	config.SetTestConfigDir(configDir)
	defer config.ResetTestWorkingDir()
	defer config.ResetTestConfigDir()

	mock := NewMockClient()
	mock.OnGet("/my/identity.json", &client.APIResponse{StatusCode: 200, Data: map[string]any{
		"id":            "user-123",
		"email_address": "doctor@example.com",
		"accounts":      []any{map[string]any{"id": "1", "slug": "/acme", "name": "Acme"}},
	}})
	mock.OnGet("/boards.json", &client.APIResponse{StatusCode: 200, Data: []any{map[string]any{"id": "board-1", "name": "Roadmap"}}})
	mock.OnGet("/boards/board-1.json", &client.APIResponse{StatusCode: 200, Data: map[string]any{"id": "board-1", "name": "Roadmap"}})

	result := SetTestModeWithSDK(mock)
	localConfig := "token: local-token\naccount: acme\napi_url: " + testHTTPServer.URL + "\nboard: board-1\n"
	if err := os.WriteFile(filepath.Join(workDir, config.LocalConfigFile), []byte(localConfig), 0o600); err != nil {
		t.Fatalf("write local config: %v", err)
	}
	cfg = config.Load()
	defer resetTest()

	cmd, _, err := rootCmd.Find([]string{"doctor"})
	if err != nil {
		t.Fatalf("find doctor command: %v", err)
	}

	err = cmd.RunE(cmd, []string{})
	assertExitCode(t, err, 0)

	data := result.Response.Data.(map[string]any)
	checks := data["checks"].([]any)
	for _, item := range checks {
		check := item.(map[string]any)
		if check["name"] == "Credential Storage" {
			if check["status"] != "warn" {
				t.Fatalf("expected Credential Storage warn, got %#v", check)
			}
			if !strings.Contains(check["message"].(string), "local project config") {
				t.Fatalf("expected local config storage warning, got %#v", check)
			}
			return
		}
	}
	t.Fatal("expected Credential Storage check in output")
}

func TestDoctorStyledOutputIncludesNextSteps(t *testing.T) {
	configDir := t.TempDir()
	config.SetTestConfigDir(configDir)
	defer config.ResetTestConfigDir()

	mock := NewMockClient()
	SetTestModeWithSDK(mock)
	SetTestConfig("", "", testHTTPServer.URL)
	SetTestFormat(output.FormatStyled)
	defer resetTest()

	cmd, _, err := rootCmd.Find([]string{"doctor"})
	if err != nil {
		t.Fatalf("find doctor command: %v", err)
	}

	err = cmd.RunE(cmd, []string{})
	assertExitCode(t, err, 0)

	raw := TestOutput()
	if !strings.Contains(raw, "Fizzy CLI Doctor") {
		t.Fatalf("expected styled title, got:\n%s", raw)
	}
	if !strings.Contains(raw, "Next steps") {
		t.Fatalf("expected next steps section, got:\n%s", raw)
	}
	if !strings.Contains(raw, "fizzy auth login <token>") {
		t.Fatalf("expected auth hint command, got:\n%s", raw)
	}
}

func TestDoctorShowsSavedProfileNames(t *testing.T) {
	configDir := t.TempDir()
	profileDir := t.TempDir()
	config.SetTestConfigDir(configDir)
	defer config.ResetTestConfigDir()

	profileStore := profile.NewStore(filepath.Join(profileDir, "config.json"))

	mock := NewMockClient()
	result := SetTestModeWithSDK(mock)
	if err := profileStore.Create(&profile.Profile{Name: "acme", BaseURL: testHTTPServer.URL}); err != nil {
		t.Fatalf("create acme profile: %v", err)
	}
	if err := profileStore.Create(&profile.Profile{Name: "staging", BaseURL: testHTTPServer.URL}); err != nil {
		t.Fatalf("create staging profile: %v", err)
	}
	if err := profileStore.SetDefault("acme"); err != nil {
		t.Fatalf("set default profile: %v", err)
	}
	SetTestProfiles(profileStore)
	SetTestConfig("", "", testHTTPServer.URL)
	defer resetTest()

	cmd, _, err := rootCmd.Find([]string{"doctor"})
	if err != nil {
		t.Fatalf("find doctor command: %v", err)
	}

	err = cmd.RunE(cmd, []string{})
	assertExitCode(t, err, 0)

	data := result.Response.Data.(map[string]any)
	checks := data["checks"].([]any)
	for _, item := range checks {
		check := item.(map[string]any)
		if check["name"] == "Saved Profiles" {
			msg := check["message"].(string)
			if !strings.Contains(msg, "acme (default)") || !strings.Contains(msg, "staging") {
				t.Fatalf("expected saved profile names in message, got %#v", check)
			}
			if !strings.Contains(check["hint"].(string), "--all-profiles") {
				t.Fatalf("expected --all-profiles hint, got %#v", check)
			}
			return
		}
	}
	t.Fatal("expected Saved Profiles check in output")
}

func TestDoctorAllProfilesIncludesPerProfileResults(t *testing.T) {
	configDir := t.TempDir()
	credDir := t.TempDir()
	profileDir := t.TempDir()
	config.SetTestConfigDir(configDir)
	defer config.ResetTestConfigDir()

	t.Setenv("FIZZY_DOCTOR_ALL_NO_KR", "1")
	store := credstore.NewStore(credstore.StoreOptions{
		ServiceName:   "fizzy-doctor-all-test",
		DisableEnvVar: "FIZZY_DOCTOR_ALL_NO_KR",
		FallbackDir:   credDir,
	})
	profileStore := profile.NewStore(filepath.Join(profileDir, "config.json"))

	mock := NewMockClient()
	mock.OnGet("/my/identity.json", &client.APIResponse{StatusCode: 200, Data: map[string]any{
		"id":            "user-123",
		"email_address": "doctor@example.com",
		"accounts":      []any{map[string]any{"id": "1", "slug": "/acme", "name": "Acme"}},
	}})
	mock.OnGet("/boards.json", &client.APIResponse{StatusCode: 200, Data: []any{map[string]any{"id": "board-1", "name": "Roadmap"}}})
	mock.OnGet("/boards/board-1.json", &client.APIResponse{StatusCode: 200, Data: map[string]any{"id": "board-1", "name": "Roadmap"}})

	result := SetTestModeWithSDK(mock)
	if err := profileStore.Create(&profile.Profile{Name: "acme", BaseURL: testHTTPServer.URL, Extra: map[string]json.RawMessage{"board": json.RawMessage(`"board-1"`)}}); err != nil {
		t.Fatalf("create acme profile: %v", err)
	}
	if err := profileStore.Create(&profile.Profile{Name: "staging", BaseURL: testHTTPServer.URL}); err != nil {
		t.Fatalf("create staging profile: %v", err)
	}
	if err := profileStore.SetDefault("acme"); err != nil {
		t.Fatalf("set default profile: %v", err)
	}
	if err := credsSaveProfileTokenForTest(store, "acme", "test-token"); err != nil {
		t.Fatalf("save profile token: %v", err)
	}
	SetTestCreds(store)
	SetTestProfiles(profileStore)
	SetTestConfig("", "", testHTTPServer.URL)
	defer resetTest()

	cmd, _, err := rootCmd.Find([]string{"doctor"})
	if err != nil {
		t.Fatalf("find doctor command: %v", err)
	}
	if err := cmd.Flags().Set("all-profiles", "true"); err != nil {
		t.Fatalf("set all-profiles flag: %v", err)
	}
	defer cmd.Flags().Set("all-profiles", "false")

	err = cmd.RunE(cmd, []string{})
	assertExitCode(t, err, 0)

	data := result.Response.Data.(map[string]any)
	profiles, ok := data["profiles"].([]any)
	if !ok {
		t.Fatalf("expected profiles array, got %#v", data["profiles"])
	}
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profile results, got %d", len(profiles))
	}

	found := map[string]map[string]string{}
	for _, item := range profiles {
		profileResult := item.(map[string]any)
		name := profileResult["name"].(string)
		checks := profileResult["checks"].([]any)
		statuses := map[string]string{}
		for _, c := range checks {
			check := c.(map[string]any)
			statuses[check["name"].(string)] = check["status"].(string)
		}
		found[name] = statuses
	}

	if found["acme"]["Authentication"] != "pass" {
		t.Fatalf("expected acme authentication to pass, got %#v", found["acme"])
	}
	if found["staging"]["Credentials"] != "fail" {
		t.Fatalf("expected staging credentials to fail, got %#v", found["staging"])
	}
}

func credsSaveProfileTokenForTest(store *credstore.Store, profileName, token string) error {
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}
	return store.Save("profile:"+profileName, data)
}
