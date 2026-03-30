package tests

import (
	"strconv"
	"testing"
	"time"

	"github.com/basecamp/fizzy-cli/e2e/harness"
)

func TestTimeList(t *testing.T) {
	h := harness.New(t)
	defer h.Cleanup.CleanupAll(h)

	boardID := createTestBoard(t, h)
	cardNumber := createTestCard(t, h, boardID)
	cardStr := strconv.Itoa(cardNumber)

	t.Run("returns list of time entries for card", func(t *testing.T) {
		result := h.Run("time", "list", "--card", cardStr)

		if result.ExitCode != harness.ExitSuccess {
			t.Errorf("expected exit code %d, got %d\nstderr: %s", harness.ExitSuccess, result.ExitCode, result.Stderr)
		}

		if result.Response == nil {
			t.Fatalf("expected JSON response, got nil\nstdout: %s", result.Stdout)
		}

		if !result.Response.Success {
			t.Error("expected success=true")
		}

		arr := result.GetDataArray()
		if arr == nil {
			t.Error("expected data to be an array")
		}
	})

	t.Run("supports --all flag", func(t *testing.T) {
		result := h.Run("time", "list", "--card", cardStr, "--all")

		if result.ExitCode != harness.ExitSuccess {
			t.Errorf("expected exit code %d, got %d", harness.ExitSuccess, result.ExitCode)
		}
	})

	t.Run("fails without --card flag", func(t *testing.T) {
		result := h.Run("time", "list")

		if result.ExitCode == harness.ExitSuccess {
			t.Error("expected non-zero exit code for missing --card")
		}
	})
}

func TestTimeCRUD(t *testing.T) {
	h := harness.New(t)
	defer h.Cleanup.CleanupAll(h)

	boardID := createTestBoard(t, h)
	cardNumber := createTestCard(t, h, boardID)
	cardStr := strconv.Itoa(cardNumber)
	today := time.Now().Format("2006-01-02")

	var entryID string

	t.Run("add time entry", func(t *testing.T) {
		result := h.Run("time", "add",
			"--card", cardStr,
			"--date", today,
			"--duration", "1:30",
			"--description", "initial work",
		)

		if result.ExitCode != harness.ExitSuccess {
			t.Fatalf("expected exit code %d, got %d\nstderr: %s\nstdout: %s",
				harness.ExitSuccess, result.ExitCode, result.Stderr, result.Stdout)
		}

		if result.Response == nil {
			t.Fatalf("expected JSON response, got nil\nstdout: %s", result.Stdout)
		}

		if !result.Response.Success {
			t.Errorf("expected success=true, error: %+v", result.Response.Error)
		}

		entryID = result.GetIDFromLocation()
		if entryID == "" {
			entryID = result.GetDataString("id")
		}
		if entryID == "" {
			t.Fatalf("expected entry ID in response (location: %s)", result.GetLocation())
		}

		h.Cleanup.AddTimeEntry(entryID, cardNumber)

		data := result.GetDataMap()
		if totalMinutes, ok := data["total_minutes"].(float64); ok {
			if totalMinutes != 90 {
				t.Errorf("expected total_minutes 90, got %v", totalMinutes)
			}
		}
	})

	t.Run("show time entry", func(t *testing.T) {
		if entryID == "" {
			t.Skip("no entry ID from add test")
		}

		result := h.Run("time", "show", entryID, "--card", cardStr)

		if result.ExitCode != harness.ExitSuccess {
			t.Errorf("expected exit code %d, got %d\nstderr: %s", harness.ExitSuccess, result.ExitCode, result.Stderr)
		}

		if !result.Response.Success {
			t.Error("expected success=true")
		}

		id := result.GetDataString("id")
		if id != entryID {
			t.Errorf("expected id %q, got %q", entryID, id)
		}
	})

	t.Run("list includes added entry", func(t *testing.T) {
		if entryID == "" {
			t.Skip("no entry ID from add test")
		}

		result := h.Run("time", "list", "--card", cardStr)

		if result.ExitCode != harness.ExitSuccess {
			t.Errorf("expected exit code %d, got %d\nstderr: %s", harness.ExitSuccess, result.ExitCode, result.Stderr)
		}

		arr := result.GetDataArray()
		if len(arr) == 0 {
			t.Error("expected at least one time entry after add")
		}
	})

	t.Run("update time entry", func(t *testing.T) {
		if entryID == "" {
			t.Skip("no entry ID from add test")
		}

		result := h.Run("time", "update", entryID,
			"--card", cardStr,
			"--duration", "2:00",
			"--description", "revised work",
		)

		if result.ExitCode != harness.ExitSuccess {
			t.Errorf("expected exit code %d, got %d\nstderr: %s", harness.ExitSuccess, result.ExitCode, result.Stderr)
		}

		if !result.Response.Success {
			t.Error("expected success=true")
		}

		// Update fetches and returns the updated entry — verify total_minutes = 120
		data := result.GetDataMap()
		if totalMinutes, ok := data["total_minutes"].(float64); ok {
			if totalMinutes != 120 {
				t.Errorf("expected total_minutes 120 after update, got %v", totalMinutes)
			}
		}
	})

	t.Run("delete time entry", func(t *testing.T) {
		if entryID == "" {
			t.Skip("no entry ID from add test")
		}

		result := h.Run("time", "delete", entryID, "--card", cardStr)

		if result.ExitCode != harness.ExitSuccess {
			t.Errorf("expected exit code %d, got %d\nstderr: %s", harness.ExitSuccess, result.ExitCode, result.Stderr)
		}

		if !result.Response.Success {
			t.Error("expected success=true")
		}

		deleted := result.GetDataBool("deleted")
		if !deleted {
			t.Error("expected deleted=true")
		}

		// Remove from cleanup since we deleted it
		if len(h.Cleanup.TimeEntries) > 0 {
			h.Cleanup.TimeEntries = h.Cleanup.TimeEntries[:len(h.Cleanup.TimeEntries)-1]
		}
	})
}

func TestTimeAddMissingFlags(t *testing.T) {
	h := harness.New(t)
	defer h.Cleanup.CleanupAll(h)

	boardID := createTestBoard(t, h)
	cardNumber := createTestCard(t, h, boardID)
	cardStr := strconv.Itoa(cardNumber)
	today := time.Now().Format("2006-01-02")

	t.Run("fails without --card", func(t *testing.T) {
		result := h.Run("time", "add", "--date", today, "--duration", "1:00")
		if result.ExitCode == harness.ExitSuccess {
			t.Error("expected failure without --card")
		}
	})

	t.Run("fails without --date", func(t *testing.T) {
		result := h.Run("time", "add", "--card", cardStr, "--duration", "1:00")
		if result.ExitCode == harness.ExitSuccess {
			t.Error("expected failure without --date")
		}
	})

	t.Run("fails without --duration", func(t *testing.T) {
		result := h.Run("time", "add", "--card", cardStr, "--date", today)
		if result.ExitCode == harness.ExitSuccess {
			t.Error("expected failure without --duration")
		}
	})

	t.Run("fails with invalid duration format", func(t *testing.T) {
		result := h.Run("time", "add", "--card", cardStr, "--date", today, "--duration", "invalid")
		if result.ExitCode == harness.ExitSuccess {
			t.Error("expected failure with invalid duration")
		}
	})
}
