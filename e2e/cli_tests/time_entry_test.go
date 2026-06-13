package clitests

import (
	"strconv"
	"testing"
	"time"

	"github.com/basecamp/fizzy-cli/e2e/harness"
)

func TestTimeList(t *testing.T) {
	h := newHarness(t)
	cardStr := strconv.Itoa(fixture.CardNumber)

	t.Run("returns list of time entries for card", func(t *testing.T) {
		result := h.Run("time", "list", "--card", cardStr)
		assertOK(t, result)
		if result.GetDataArray() == nil {
			t.Error("expected data to be an array")
		}
	})

	t.Run("supports --all flag", func(t *testing.T) {
		assertOK(t, h.Run("time", "list", "--card", cardStr, "--all"))
	})

	t.Run("fails without --card flag", func(t *testing.T) {
		result := h.Run("time", "list")
		if result.ExitCode == harness.ExitSuccess {
			t.Error("expected non-zero exit code for missing --card")
		}
	})
}

func TestTimeCRUD(t *testing.T) {
	h := newHarness(t)
	cardNumber := createCard(t, h, fixture.BoardID)
	cardStr := strconv.Itoa(cardNumber)
	today := time.Now().Format("2006-01-02")

	var entryID string
	t.Cleanup(func() {
		if entryID != "" {
			newHarness(t).Run("time", "delete", entryID, "--card", cardStr)
		}
	})

	t.Run("add time entry", func(t *testing.T) {
		result := h.Run("time", "add",
			"--card", cardStr,
			"--date", today,
			"--duration", "1:30",
			"--description", "initial work",
		)
		assertOK(t, result)

		entryID = result.GetIDFromLocation()
		if entryID == "" {
			entryID = result.GetDataString("id")
		}
		if entryID == "" {
			t.Fatalf("expected entry ID in response (location: %s)", result.GetLocation())
		}

		data := result.GetDataMap()
		if totalMinutes, ok := data["total_minutes"].(float64); ok && totalMinutes != 90 {
			t.Errorf("expected total_minutes 90, got %v", totalMinutes)
		}
	})

	t.Run("show time entry", func(t *testing.T) {
		if entryID == "" {
			t.Skip("no entry ID from add test")
		}
		result := h.Run("time", "show", entryID, "--card", cardStr)
		assertOK(t, result)
		if got := result.GetDataString("id"); got != entryID {
			t.Errorf("expected id %q, got %q", entryID, got)
		}
	})

	t.Run("list includes added entry", func(t *testing.T) {
		if entryID == "" {
			t.Skip("no entry ID from add test")
		}
		result := h.Run("time", "list", "--card", cardStr)
		assertOK(t, result)
		if len(result.GetDataArray()) == 0 {
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
		assertOK(t, result)

		data := result.GetDataMap()
		if totalMinutes, ok := data["total_minutes"].(float64); ok && totalMinutes != 120 {
			t.Errorf("expected total_minutes 120 after update, got %v", totalMinutes)
		}
	})

	t.Run("delete time entry", func(t *testing.T) {
		if entryID == "" {
			t.Skip("no entry ID from add test")
		}
		result := h.Run("time", "delete", entryID, "--card", cardStr)
		assertOK(t, result)
		if !result.GetDataBool("deleted") {
			t.Error("expected deleted=true")
		}
		entryID = ""
	})
}

func TestTimeAddMissingFlags(t *testing.T) {
	h := newHarness(t)
	cardStr := strconv.Itoa(fixture.CardNumber)
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
