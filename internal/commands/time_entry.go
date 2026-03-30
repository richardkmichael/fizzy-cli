package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/basecamp/fizzy-cli/internal/errors"
	"github.com/spf13/cobra"
)

var timeCmd = &cobra.Command{
	Use:   "time",
	Short: "Manage time entries",
	Long:  "Commands for managing card time entries.",
}

// parseDuration parses "HH:MM" (e.g. "1:30") into hours and minutes.
func parseDuration(s string) (int, int, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid duration format %q: expected HH:MM (e.g. 1:30)", s)
	}
	hours, err := strconv.Atoi(parts[0])
	if err != nil || hours < 0 {
		return 0, 0, fmt.Errorf("invalid duration format %q: hours must be a non-negative integer", s)
	}
	minutes, err := strconv.Atoi(parts[1])
	if err != nil || minutes < 0 || minutes > 59 {
		return 0, 0, fmt.Errorf("invalid duration format %q: minutes must be 0-59", s)
	}
	if hours == 0 && minutes == 0 {
		return 0, 0, fmt.Errorf("duration must be greater than zero")
	}
	return hours, minutes, nil
}

// time list flags
var timeListCard string
var timeListPage int
var timeListAll bool

var timeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List time entries for a card",
	Long:  "Lists all time entries for a specific card.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		if timeListCard == "" {
			return newRequiredFlagError("card")
		}

		c := getClient()
		path := "/cards/" + timeListCard + "/time_entries.json"
		if timeListPage > 0 {
			path += "?page=" + strconv.Itoa(timeListPage)
		}

		resp, err := c.GetWithPagination(path, timeListAll)
		if err != nil {
			return err
		}

		count := 0
		if arr, ok := resp.Data.([]interface{}); ok {
			count = len(arr)
		}
		summary := fmt.Sprintf("%d time entries on card #%s", count, timeListCard)
		if timeListAll {
			summary += " (all)"
		} else if timeListPage > 0 {
			summary += fmt.Sprintf(" (page %d)", timeListPage)
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("add", fmt.Sprintf("fizzy time add --card %s --date <date> --duration <HH:MM>", timeListCard), "Log time"),
			breadcrumb("card", fmt.Sprintf("fizzy card show %s", timeListCard), "View card"),
		}

		hasNext := resp.LinkNext != ""
		printListPaginated(resp.Data, nil, hasNext, resp.LinkNext, timeListAll, summary, breadcrumbs)
		return nil
	},
}

// time show flags
var timeShowCard string

var timeShowCmd = &cobra.Command{
	Use:   "show TIME_ENTRY_ID",
	Short: "Show a time entry",
	Long:  "Shows details of a specific time entry.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		if timeShowCard == "" {
			return newRequiredFlagError("card")
		}

		entryID := args[0]
		cardNumber := timeShowCard

		c := getClient()
		resp, err := c.Get("/cards/" + cardNumber + "/time_entries/" + entryID + ".json")
		if err != nil {
			return err
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("update", fmt.Sprintf("fizzy time update %s --card %s", entryID, cardNumber), "Edit time entry"),
			breadcrumb("delete", fmt.Sprintf("fizzy time delete %s --card %s", entryID, cardNumber), "Delete time entry"),
			breadcrumb("list", fmt.Sprintf("fizzy time list --card %s", cardNumber), "List time entries"),
		}

		printDetail(resp.Data, "", breadcrumbs)
		return nil
	},
}

// postTimeEntry is shared logic for the add and remove subcommands.
// Both POST to the same endpoint; commit is "add" or "remove".
func postTimeEntry(cardNumber, date, duration, description, commit string) error {
	hours, minutes, err := parseDuration(duration)
	if err != nil {
		return errors.NewInvalidArgsError(err.Error())
	}

	entryParams := map[string]interface{}{
		"hours":   hours,
		"minutes": minutes,
		"date":    date,
	}
	if description != "" {
		entryParams["description"] = description
	}

	reqBody := map[string]interface{}{
		"time_entry": entryParams,
		"commit":     commit,
	}

	c := getClient()
	resp, err := c.Post("/cards/"+cardNumber+"/time_entries.json", reqBody)
	if err != nil {
		return err
	}

	breadcrumbs := []Breadcrumb{
		breadcrumb("list", fmt.Sprintf("fizzy time list --card %s", cardNumber), "List time entries"),
		breadcrumb("card", fmt.Sprintf("fizzy card show %s", cardNumber), "View card"),
	}

	if resp.Location != "" {
		followResp, err := c.FollowLocation(resp.Location)
		if err == nil && followResp != nil {
			entryID := ""
			if entry, ok := followResp.Data.(map[string]interface{}); ok {
				if id, ok := entry["id"].(string); ok {
					entryID = id
				}
			}
			if entryID != "" {
				breadcrumbs = append([]Breadcrumb{
					breadcrumb("view", fmt.Sprintf("fizzy time show %s --card %s", entryID, cardNumber), "View time entry"),
				}, breadcrumbs...)
			}

			printMutationWithLocation(followResp.Data, resp.Location, breadcrumbs)
			return nil
		}
		printSuccessWithLocation(resp.Location)
		return nil
	}

	printSuccessWithBreadcrumbs(resp.Data, "", breadcrumbs)
	return nil
}

// time add flags
var timeAddCard string
var timeAddDate string
var timeAddDuration string
var timeAddDescription string

var timeAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Log time on a card",
	Long:  "Logs time worked on a card.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		if timeAddCard == "" {
			return newRequiredFlagError("card")
		}
		if timeAddDate == "" {
			return newRequiredFlagError("date")
		}
		if timeAddDuration == "" {
			return newRequiredFlagError("duration")
		}

		return postTimeEntry(timeAddCard, timeAddDate, timeAddDuration, timeAddDescription, "add")
	},
}

// time remove flags
var timeRemoveCard string
var timeRemoveDate string
var timeRemoveDuration string
var timeRemoveDescription string

var timeRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove logged time from a card",
	Long:  "Removes previously logged time from a card.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		if timeRemoveCard == "" {
			return newRequiredFlagError("card")
		}
		if timeRemoveDate == "" {
			return newRequiredFlagError("date")
		}
		if timeRemoveDuration == "" {
			return newRequiredFlagError("duration")
		}

		return postTimeEntry(timeRemoveCard, timeRemoveDate, timeRemoveDuration, timeRemoveDescription, "remove")
	},
}

// time update flags
var timeUpdateCard string
var timeUpdateDate string
var timeUpdateDuration string
var timeUpdateDescription string

var timeUpdateCmd = &cobra.Command{
	Use:   "update TIME_ENTRY_ID",
	Short: "Update a time entry",
	Long:  "Updates an existing time entry.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		if timeUpdateCard == "" {
			return newRequiredFlagError("card")
		}

		entryID := args[0]
		cardNumber := timeUpdateCard

		entryParams := make(map[string]interface{})
		if timeUpdateDate != "" {
			entryParams["date"] = timeUpdateDate
		}
		if timeUpdateDuration != "" {
			hours, minutes, err := parseDuration(timeUpdateDuration)
			if err != nil {
				return errors.NewInvalidArgsError(err.Error())
			}
			entryParams["hours"] = hours
			entryParams["minutes"] = minutes
		}
		if timeUpdateDescription != "" {
			entryParams["description"] = timeUpdateDescription
		}

		reqBody := map[string]interface{}{
			"time_entry": entryParams,
		}

		c := getClient()
		_, err := c.Patch("/cards/"+cardNumber+"/time_entries/"+entryID+".json", reqBody)
		if err != nil {
			return err
		}

		// Update returns 204 No Content — fetch the entry to return it.
		resp, err := c.Get("/cards/" + cardNumber + "/time_entries/" + entryID + ".json")
		if err != nil {
			return err
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("show", fmt.Sprintf("fizzy time show %s --card %s", entryID, cardNumber), "View time entry"),
			breadcrumb("list", fmt.Sprintf("fizzy time list --card %s", cardNumber), "List time entries"),
		}

		printMutation(resp.Data, "", breadcrumbs)
		return nil
	},
}

// time delete flags
var timeDeleteCard string

var timeDeleteCmd = &cobra.Command{
	Use:   "delete TIME_ENTRY_ID",
	Short: "Delete a time entry",
	Long:  "Deletes a time entry from a card.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		if timeDeleteCard == "" {
			return newRequiredFlagError("card")
		}

		cardNumber := timeDeleteCard

		c := getClient()
		_, err := c.Delete("/cards/" + cardNumber + "/time_entries/" + args[0] + ".json")
		if err != nil {
			return err
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("list", fmt.Sprintf("fizzy time list --card %s", cardNumber), "List time entries"),
			breadcrumb("card", fmt.Sprintf("fizzy card show %s", cardNumber), "View card"),
		}

		printMutation(map[string]any{
			"deleted": true,
		}, "", breadcrumbs)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(timeCmd)

	// List
	timeListCmd.Flags().StringVar(&timeListCard, "card", "", "Card number (required)")
	timeListCmd.Flags().IntVar(&timeListPage, "page", 0, "Page number")
	timeListCmd.Flags().BoolVar(&timeListAll, "all", false, "Fetch all pages")
	timeCmd.AddCommand(timeListCmd)

	// Show
	timeShowCmd.Flags().StringVar(&timeShowCard, "card", "", "Card number (required)")
	timeCmd.AddCommand(timeShowCmd)

	// Add
	timeAddCmd.Flags().StringVar(&timeAddCard, "card", "", "Card number (required)")
	timeAddCmd.Flags().StringVar(&timeAddDate, "date", "", "Date of work in YYYY-MM-DD format (required)")
	timeAddCmd.Flags().StringVar(&timeAddDuration, "duration", "", "Duration as HH:MM, e.g. 1:30 (required)")
	timeAddCmd.Flags().StringVar(&timeAddDescription, "description", "", "Description of work done")
	timeCmd.AddCommand(timeAddCmd)

	// Remove
	timeRemoveCmd.Flags().StringVar(&timeRemoveCard, "card", "", "Card number (required)")
	timeRemoveCmd.Flags().StringVar(&timeRemoveDate, "date", "", "Date of work in YYYY-MM-DD format (required)")
	timeRemoveCmd.Flags().StringVar(&timeRemoveDuration, "duration", "", "Duration as HH:MM, e.g. 1:30 (required)")
	timeRemoveCmd.Flags().StringVar(&timeRemoveDescription, "description", "", "Description")
	timeCmd.AddCommand(timeRemoveCmd)

	// Update
	timeUpdateCmd.Flags().StringVar(&timeUpdateCard, "card", "", "Card number (required)")
	timeUpdateCmd.Flags().StringVar(&timeUpdateDate, "date", "", "Date of work in YYYY-MM-DD format")
	timeUpdateCmd.Flags().StringVar(&timeUpdateDuration, "duration", "", "Duration as HH:MM, e.g. 1:30")
	timeUpdateCmd.Flags().StringVar(&timeUpdateDescription, "description", "", "Description of work done")
	timeCmd.AddCommand(timeUpdateCmd)

	// Delete
	timeDeleteCmd.Flags().StringVar(&timeDeleteCard, "card", "", "Card number (required)")
	timeCmd.AddCommand(timeDeleteCmd)
}
