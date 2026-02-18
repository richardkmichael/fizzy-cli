package commands

import (
	"testing"

	"github.com/robzolkos/fizzy-cli/internal/client"
	"github.com/robzolkos/fizzy-cli/internal/errors"
)

func TestParseDuration(t *testing.T) {
	t.Run("parses valid HH:MM", func(t *testing.T) {
		hours, minutes, err := parseDuration("1:30")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hours != 1 {
			t.Errorf("expected hours 1, got %d", hours)
		}
		if minutes != 30 {
			t.Errorf("expected minutes 30, got %d", minutes)
		}
	})

	t.Run("parses zero hours", func(t *testing.T) {
		hours, minutes, err := parseDuration("0:45")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hours != 0 {
			t.Errorf("expected hours 0, got %d", hours)
		}
		if minutes != 45 {
			t.Errorf("expected minutes 45, got %d", minutes)
		}
	})

	t.Run("rejects missing colon", func(t *testing.T) {
		_, _, err := parseDuration("130")
		if err == nil {
			t.Error("expected error for missing colon")
		}
	})

	t.Run("rejects minutes out of range", func(t *testing.T) {
		_, _, err := parseDuration("1:60")
		if err == nil {
			t.Error("expected error for minutes >= 60")
		}
	})

	t.Run("rejects zero duration", func(t *testing.T) {
		_, _, err := parseDuration("0:00")
		if err == nil {
			t.Error("expected error for zero duration")
		}
	})

	t.Run("rejects negative hours", func(t *testing.T) {
		_, _, err := parseDuration("-1:30")
		if err == nil {
			t.Error("expected error for negative hours")
		}
	})
}

func TestTimeList(t *testing.T) {
	t.Run("lists time entries for card", func(t *testing.T) {
		mock := NewMockClient()
		mock.GetWithPaginationResponse = &client.APIResponse{
			StatusCode: 200,
			Data: []interface{}{
				map[string]interface{}{"id": "entry-1", "total_minutes": float64(90)},
				map[string]interface{}{"id": "entry-2", "total_minutes": float64(60)},
			},
		}

		result := SetTestMode(mock)
		SetTestConfig("token", "account", "https://api.example.com")
		defer ResetTestMode()

		timeListCard = "42"
		RunTestCommand(func() {
			timeListCmd.Run(timeListCmd, []string{})
		})
		timeListCard = ""

		if result.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", result.ExitCode)
		}
		if mock.GetWithPaginationCalls[0].Path != "/cards/42/time_entries.json" {
			t.Errorf("expected path '/cards/42/time_entries.json', got '%s'", mock.GetWithPaginationCalls[0].Path)
		}
	})

	t.Run("requires --card flag", func(t *testing.T) {
		mock := NewMockClient()
		result := SetTestMode(mock)
		SetTestConfig("token", "account", "https://api.example.com")
		defer ResetTestMode()

		timeListCard = ""
		RunTestCommand(func() {
			timeListCmd.Run(timeListCmd, []string{})
		})

		if result.ExitCode != errors.ExitInvalidArgs {
			t.Errorf("expected exit code %d, got %d", errors.ExitInvalidArgs, result.ExitCode)
		}
	})
}

func TestTimeShow(t *testing.T) {
	t.Run("shows time entry by ID", func(t *testing.T) {
		mock := NewMockClient()
		mock.GetResponse = &client.APIResponse{
			StatusCode: 200,
			Data: map[string]interface{}{
				"id":            "entry-1",
				"total_minutes": float64(90),
				"date":          "2026-02-18",
			},
		}

		result := SetTestMode(mock)
		SetTestConfig("token", "account", "https://api.example.com")
		defer ResetTestMode()

		timeShowCard = "42"
		RunTestCommand(func() {
			timeShowCmd.Run(timeShowCmd, []string{"entry-1"})
		})
		timeShowCard = ""

		if result.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", result.ExitCode)
		}
		if mock.GetCalls[0].Path != "/cards/42/time_entries/entry-1.json" {
			t.Errorf("expected path '/cards/42/time_entries/entry-1.json', got '%s'", mock.GetCalls[0].Path)
		}
	})

	t.Run("requires --card flag", func(t *testing.T) {
		mock := NewMockClient()
		result := SetTestMode(mock)
		SetTestConfig("token", "account", "https://api.example.com")
		defer ResetTestMode()

		timeShowCard = ""
		RunTestCommand(func() {
			timeShowCmd.Run(timeShowCmd, []string{"entry-1"})
		})

		if result.ExitCode != errors.ExitInvalidArgs {
			t.Errorf("expected exit code %d, got %d", errors.ExitInvalidArgs, result.ExitCode)
		}
	})
}

func TestTimeAdd(t *testing.T) {
	t.Run("logs time and follows location", func(t *testing.T) {
		mock := NewMockClient()
		mock.PostResponse = &client.APIResponse{
			StatusCode: 201,
			Location:   "https://api.example.com/cards/42/time_entries/entry-1",
		}
		mock.FollowLocationResponse = &client.APIResponse{
			StatusCode: 200,
			Data: map[string]interface{}{
				"id":            "entry-1",
				"total_minutes": float64(90),
			},
		}

		result := SetTestMode(mock)
		SetTestConfig("token", "account", "https://api.example.com")
		defer ResetTestMode()

		timeAddCard = "42"
		timeAddDate = "2026-02-18"
		timeAddDuration = "1:30"
		timeAddDescription = "planning work"
		RunTestCommand(func() {
			timeAddCmd.Run(timeAddCmd, []string{})
		})
		timeAddCard = ""
		timeAddDate = ""
		timeAddDuration = ""
		timeAddDescription = ""

		if result.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", result.ExitCode)
		}
		if mock.PostCalls[0].Path != "/cards/42/time_entries.json" {
			t.Errorf("expected path '/cards/42/time_entries.json', got '%s'", mock.PostCalls[0].Path)
		}

		body := mock.PostCalls[0].Body.(map[string]interface{})
		if body["commit"] != "add" {
			t.Errorf("expected commit 'add', got '%v'", body["commit"])
		}
		entry := body["time_entry"].(map[string]interface{})
		if entry["hours"] != 1 {
			t.Errorf("expected hours 1, got %v", entry["hours"])
		}
		if entry["minutes"] != 30 {
			t.Errorf("expected minutes 30, got %v", entry["minutes"])
		}
		if entry["date"] != "2026-02-18" {
			t.Errorf("expected date '2026-02-18', got '%v'", entry["date"])
		}
		if entry["description"] != "planning work" {
			t.Errorf("expected description 'planning work', got '%v'", entry["description"])
		}
	})

	t.Run("requires --card flag", func(t *testing.T) {
		mock := NewMockClient()
		result := SetTestMode(mock)
		SetTestConfig("token", "account", "https://api.example.com")
		defer ResetTestMode()

		timeAddCard = ""
		timeAddDate = "2026-02-18"
		timeAddDuration = "1:00"
		RunTestCommand(func() {
			timeAddCmd.Run(timeAddCmd, []string{})
		})
		timeAddDate = ""
		timeAddDuration = ""

		if result.ExitCode != errors.ExitInvalidArgs {
			t.Errorf("expected exit code %d, got %d", errors.ExitInvalidArgs, result.ExitCode)
		}
	})

	t.Run("requires --date flag", func(t *testing.T) {
		mock := NewMockClient()
		result := SetTestMode(mock)
		SetTestConfig("token", "account", "https://api.example.com")
		defer ResetTestMode()

		timeAddCard = "42"
		timeAddDate = ""
		timeAddDuration = "1:00"
		RunTestCommand(func() {
			timeAddCmd.Run(timeAddCmd, []string{})
		})
		timeAddCard = ""
		timeAddDuration = ""

		if result.ExitCode != errors.ExitInvalidArgs {
			t.Errorf("expected exit code %d, got %d", errors.ExitInvalidArgs, result.ExitCode)
		}
	})

	t.Run("requires --duration flag", func(t *testing.T) {
		mock := NewMockClient()
		result := SetTestMode(mock)
		SetTestConfig("token", "account", "https://api.example.com")
		defer ResetTestMode()

		timeAddCard = "42"
		timeAddDate = "2026-02-18"
		timeAddDuration = ""
		RunTestCommand(func() {
			timeAddCmd.Run(timeAddCmd, []string{})
		})
		timeAddCard = ""
		timeAddDate = ""

		if result.ExitCode != errors.ExitInvalidArgs {
			t.Errorf("expected exit code %d, got %d", errors.ExitInvalidArgs, result.ExitCode)
		}
	})

	t.Run("rejects invalid duration format", func(t *testing.T) {
		mock := NewMockClient()
		result := SetTestMode(mock)
		SetTestConfig("token", "account", "https://api.example.com")
		defer ResetTestMode()

		timeAddCard = "42"
		timeAddDate = "2026-02-18"
		timeAddDuration = "invalid"
		RunTestCommand(func() {
			timeAddCmd.Run(timeAddCmd, []string{})
		})
		timeAddCard = ""
		timeAddDate = ""
		timeAddDuration = ""

		if result.ExitCode != errors.ExitInvalidArgs {
			t.Errorf("expected exit code %d, got %d", errors.ExitInvalidArgs, result.ExitCode)
		}
	})
}

func TestTimeRemove(t *testing.T) {
	t.Run("sends commit=remove", func(t *testing.T) {
		mock := NewMockClient()
		mock.PostResponse = &client.APIResponse{
			StatusCode: 201,
			Location:   "https://api.example.com/cards/42/time_entries/entry-2",
		}
		mock.FollowLocationResponse = &client.APIResponse{
			StatusCode: 200,
			Data:       map[string]interface{}{"id": "entry-2", "total_minutes": float64(-60)},
		}

		result := SetTestMode(mock)
		SetTestConfig("token", "account", "https://api.example.com")
		defer ResetTestMode()

		timeRemoveCard = "42"
		timeRemoveDate = "2026-02-18"
		timeRemoveDuration = "1:00"
		RunTestCommand(func() {
			timeRemoveCmd.Run(timeRemoveCmd, []string{})
		})
		timeRemoveCard = ""
		timeRemoveDate = ""
		timeRemoveDuration = ""

		if result.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", result.ExitCode)
		}
		body := mock.PostCalls[0].Body.(map[string]interface{})
		if body["commit"] != "remove" {
			t.Errorf("expected commit 'remove', got '%v'", body["commit"])
		}
		entry := body["time_entry"].(map[string]interface{})
		if entry["hours"] != 1 {
			t.Errorf("expected hours 1, got %v", entry["hours"])
		}
		if entry["minutes"] != 0 {
			t.Errorf("expected minutes 0, got %v", entry["minutes"])
		}
	})
}

func TestTimeUpdate(t *testing.T) {
	t.Run("patches entry and fetches result", func(t *testing.T) {
		mock := NewMockClient()
		mock.PatchResponse = &client.APIResponse{
			StatusCode: 204,
		}
		mock.GetResponse = &client.APIResponse{
			StatusCode: 200,
			Data: map[string]interface{}{
				"id":            "entry-1",
				"total_minutes": float64(120),
				"date":          "2026-02-18",
			},
		}

		result := SetTestMode(mock)
		SetTestConfig("token", "account", "https://api.example.com")
		defer ResetTestMode()

		timeUpdateCard = "42"
		timeUpdateDuration = "2:00"
		RunTestCommand(func() {
			timeUpdateCmd.Run(timeUpdateCmd, []string{"entry-1"})
		})
		timeUpdateCard = ""
		timeUpdateDuration = ""

		if result.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", result.ExitCode)
		}
		if mock.PatchCalls[0].Path != "/cards/42/time_entries/entry-1.json" {
			t.Errorf("expected patch path '/cards/42/time_entries/entry-1.json', got '%s'", mock.PatchCalls[0].Path)
		}
		if mock.GetCalls[0].Path != "/cards/42/time_entries/entry-1.json" {
			t.Errorf("expected get path '/cards/42/time_entries/entry-1.json', got '%s'", mock.GetCalls[0].Path)
		}

		body := mock.PatchCalls[0].Body.(map[string]interface{})
		entry := body["time_entry"].(map[string]interface{})
		if entry["hours"] != 2 {
			t.Errorf("expected hours 2, got %v", entry["hours"])
		}
		if entry["minutes"] != 0 {
			t.Errorf("expected minutes 0, got %v", entry["minutes"])
		}
	})

	t.Run("requires --card flag", func(t *testing.T) {
		mock := NewMockClient()
		result := SetTestMode(mock)
		SetTestConfig("token", "account", "https://api.example.com")
		defer ResetTestMode()

		timeUpdateCard = ""
		RunTestCommand(func() {
			timeUpdateCmd.Run(timeUpdateCmd, []string{"entry-1"})
		})

		if result.ExitCode != errors.ExitInvalidArgs {
			t.Errorf("expected exit code %d, got %d", errors.ExitInvalidArgs, result.ExitCode)
		}
	})

	t.Run("rejects invalid duration format", func(t *testing.T) {
		mock := NewMockClient()
		result := SetTestMode(mock)
		SetTestConfig("token", "account", "https://api.example.com")
		defer ResetTestMode()

		timeUpdateCard = "42"
		timeUpdateDuration = "bad"
		RunTestCommand(func() {
			timeUpdateCmd.Run(timeUpdateCmd, []string{"entry-1"})
		})
		timeUpdateCard = ""
		timeUpdateDuration = ""

		if result.ExitCode != errors.ExitInvalidArgs {
			t.Errorf("expected exit code %d, got %d", errors.ExitInvalidArgs, result.ExitCode)
		}
	})
}

func TestTimeDelete(t *testing.T) {
	t.Run("deletes time entry", func(t *testing.T) {
		mock := NewMockClient()
		mock.DeleteResponse = &client.APIResponse{
			StatusCode: 204,
			Data:       map[string]interface{}{},
		}

		result := SetTestMode(mock)
		SetTestConfig("token", "account", "https://api.example.com")
		defer ResetTestMode()

		timeDeleteCard = "42"
		RunTestCommand(func() {
			timeDeleteCmd.Run(timeDeleteCmd, []string{"entry-1"})
		})
		timeDeleteCard = ""

		if result.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", result.ExitCode)
		}
		if mock.DeleteCalls[0].Path != "/cards/42/time_entries/entry-1.json" {
			t.Errorf("expected path '/cards/42/time_entries/entry-1.json', got '%s'", mock.DeleteCalls[0].Path)
		}
	})

	t.Run("requires --card flag", func(t *testing.T) {
		mock := NewMockClient()
		result := SetTestMode(mock)
		SetTestConfig("token", "account", "https://api.example.com")
		defer ResetTestMode()

		timeDeleteCard = ""
		RunTestCommand(func() {
			timeDeleteCmd.Run(timeDeleteCmd, []string{"entry-1"})
		})

		if result.ExitCode != errors.ExitInvalidArgs {
			t.Errorf("expected exit code %d, got %d", errors.ExitInvalidArgs, result.ExitCode)
		}
	})
}
