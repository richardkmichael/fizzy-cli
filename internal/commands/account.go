package commands

import (
	"fmt"

	"github.com/basecamp/fizzy-sdk/go/pkg/generated"
	"github.com/spf13/cobra"
)

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Manage account settings",
	Long:  "Commands for managing account settings.",
}

var accountShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show account settings",
	Long:  "Shows the current account settings including auto-postpone period.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		data, _, err := getSDK().Account().GetSettings(cmd.Context())
		if err != nil {
			return convertSDKError(err)
		}

		items := normalizeAny(data)

		summary := "Account"
		if account, ok := items.(map[string]any); ok {
			if name, ok := account["name"].(string); ok && name != "" {
				summary = fmt.Sprintf("Account: %s", name)
			}
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("boards", "fizzy board list", "List boards"),
			breadcrumb("entropy", "fizzy account entropy --auto_postpone_period_in_days N", "Update auto-postpone period"),
			breadcrumb("settings-update", "fizzy account settings-update --name \"name\"", "Update settings"),
		}

		printDetail(items, summary, breadcrumbs)
		return nil
	},
}

// Account entropy flags
var accountEntropyAutoPostponePeriodInDays int

var accountEntropyCmd = &cobra.Command{
	Use:   "entropy",
	Short: "Update account auto-postpone period",
	Long:  "Updates the account-level default auto-postpone period. Requires admin role.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		if accountEntropyAutoPostponePeriodInDays == 0 {
			return newRequiredFlagError("auto_postpone_period_in_days")
		}
		if err := validateAutoPostponePeriodInDays(accountEntropyAutoPostponePeriodInDays); err != nil {
			return err
		}

		req := &generated.UpdateAccountEntropyRequest{
			AutoPostponePeriodInDays: int32(accountEntropyAutoPostponePeriodInDays),
		}

		data, _, err := getSDK().Account().UpdateEntropy(cmd.Context(), req)
		if err != nil {
			return convertSDKError(err)
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("show", "fizzy account show", "View account settings"),
			breadcrumb("boards", "fizzy board list", "List boards"),
		}

		printMutation(normalizeAny(data), "", breadcrumbs)
		return nil
	},
}

// Account settings update flags
var accountSettingsUpdateName string

var accountSettingsUpdateCmd = &cobra.Command{
	Use:   "settings-update",
	Short: "Update account settings",
	Long:  "Updates account settings.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		if accountSettingsUpdateName == "" {
			return newRequiredFlagError("name")
		}

		_, err := getSDK().Account().UpdateSettings(cmd.Context(), &generated.UpdateAccountSettingsRequest{
			Name: accountSettingsUpdateName,
		})
		if err != nil {
			return convertSDKError(err)
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("show", "fizzy account show", "View account settings"),
		}

		printMutation(map[string]any{}, "", breadcrumbs)
		return nil
	},
}

var accountExportCreateCmd = &cobra.Command{
	Use:   "export-create",
	Short: "Create an account export",
	Long:  "Creates a new account data export.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		data, _, err := getSDK().Account().CreateExport(cmd.Context())
		if err != nil {
			return convertSDKError(err)
		}

		items := normalizeAny(data)

		exportID := ""
		if export, ok := items.(map[string]any); ok {
			if id, ok := export["id"]; ok {
				exportID = fmt.Sprintf("%v", id)
			}
		}

		var breadcrumbs []Breadcrumb
		if exportID != "" {
			breadcrumbs = []Breadcrumb{
				breadcrumb("show", fmt.Sprintf("fizzy account export-show %s", exportID), "View export status"),
			}
		}

		printMutation(items, "", breadcrumbs)
		return nil
	},
}

var accountExportShowCmd = &cobra.Command{
	Use:   "export-show EXPORT_ID",
	Short: "Show an account export",
	Long:  "Shows the status of an account data export.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		data, _, err := getSDK().Account().GetExport(cmd.Context(), args[0])
		if err != nil {
			return convertSDKError(err)
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("show", "fizzy account show", "View account settings"),
		}

		printDetail(normalizeAny(data), "", breadcrumbs)
		return nil
	},
}

var accountJoinCodeShowCmd = &cobra.Command{
	Use:   "join-code-show",
	Short: "Show the account join code",
	Long:  "Shows the current join code for inviting new members.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		data, _, err := getSDK().Account().GetJoinCode(cmd.Context())
		if err != nil {
			return convertSDKError(err)
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("reset", "fizzy account join-code-reset", "Reset join code"),
			breadcrumb("show", "fizzy account show", "View account settings"),
		}

		printDetail(normalizeAny(data), "", breadcrumbs)
		return nil
	},
}

var accountJoinCodeResetCmd = &cobra.Command{
	Use:   "join-code-reset",
	Short: "Reset the account join code",
	Long:  "Resets the join code, invalidating the previous one.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		_, err := getSDK().Account().ResetJoinCode(cmd.Context())
		if err != nil {
			return convertSDKError(err)
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("show", "fizzy account join-code-show", "View new join code"),
		}

		printMutation(map[string]any{}, "", breadcrumbs)
		return nil
	},
}

// Account join code update flags
var accountJoinCodeUpdateUsageLimit int

var accountJoinCodeUpdateCmd = &cobra.Command{
	Use:   "join-code-update",
	Short: "Update the account join code",
	Long:  "Updates the join code settings.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAuthAndAccount(); err != nil {
			return err
		}

		if !cmd.Flags().Changed("usage-limit") {
			return newRequiredFlagError("usage-limit")
		}

		_, err := getSDK().Account().UpdateJoinCode(cmd.Context(), &generated.UpdateJoinCodeRequest{
			UsageLimit: int32(accountJoinCodeUpdateUsageLimit),
		})
		if err != nil {
			return convertSDKError(err)
		}

		breadcrumbs := []Breadcrumb{
			breadcrumb("show", "fizzy account join-code-show", "View join code"),
		}

		printMutation(map[string]any{}, "", breadcrumbs)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(accountCmd)

	// Show
	accountCmd.AddCommand(accountShowCmd)

	// Entropy
	accountEntropyCmd.Flags().IntVar(&accountEntropyAutoPostponePeriodInDays, "auto_postpone_period_in_days", 0, "Auto postpone period in days ("+validAutoPostponePeriodsHelp+")")
	accountCmd.AddCommand(accountEntropyCmd)

	// Settings update
	accountSettingsUpdateCmd.Flags().StringVar(&accountSettingsUpdateName, "name", "", "Account name (required)")
	accountCmd.AddCommand(accountSettingsUpdateCmd)

	// Exports
	accountCmd.AddCommand(accountExportCreateCmd)
	accountCmd.AddCommand(accountExportShowCmd)

	// Join code
	accountCmd.AddCommand(accountJoinCodeShowCmd)
	accountCmd.AddCommand(accountJoinCodeResetCmd)
	accountJoinCodeUpdateCmd.Flags().IntVar(&accountJoinCodeUpdateUsageLimit, "usage-limit", 0, "Usage limit (required)")
	accountCmd.AddCommand(accountJoinCodeUpdateCmd)
}
