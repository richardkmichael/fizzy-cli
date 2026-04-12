package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/basecamp/cli/output"
	"github.com/basecamp/fizzy-cli/internal/errors"
	"github.com/spf13/cobra"
)

const (
	selfUpdateRepoOwner = "richardkmichael"
	selfUpdateRepoName  = "fizzy-cli"
)

// ghAPIBase is the GitHub API root. Overridable in tests.
var ghAPIBase = "https://api.github.com"

// executableResolver returns the path of the running binary.
// Overridable in tests so the command can be exercised without touching the
// actual test binary.
var executableResolver = resolveExecutable

// httpClient is the HTTP client used for release/asset fetches.
// Overridable in tests for custom transports.
var httpClient = &http.Client{Timeout: 300 * time.Second}

var (
	selfUpdateCheck bool
	selfUpdateForce bool
)

var selfUpdateCmd = &cobra.Command{
	Use:   "self-update",
	Short: "Download and install the latest fork release",
	Long: `Downloads the latest release asset for this platform from the fork repo on ` +
		`GitHub, verifies its SHA256 checksum, and atomically replaces the running ` +
		`binary.`,
	Example: "  $ fizzy self-update\n  $ fizzy self-update --check",
	RunE:    runSelfUpdate,
}

func init() {
	selfUpdateCmd.Flags().BoolVar(&selfUpdateCheck, "check", false,
		"check for an update without installing")
	selfUpdateCmd.Flags().BoolVar(&selfUpdateForce, "force", false,
		"re-install even if already at the latest version")
	rootCmd.AddCommand(selfUpdateCmd)
}

func runSelfUpdate(cmd *cobra.Command, args []string) error {
	if cfgJQ != "" {
		return errors.ErrJQNotSupported("the self-update command")
	}

	asset, ok := assetNameFor(runtime.GOOS, runtime.GOARCH)
	if !ok {
		return errors.NewInvalidArgsError(
			fmt.Sprintf("self-update is not supported on %s/%s", runtime.GOOS, runtime.GOARCH))
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	latest, err := fetchLatestRelease(ctx)
	if err != nil {
		return err
	}

	assetURL, hasAsset := latest.Assets[asset]
	sumsURL, hasSums := latest.Assets["checksums.txt"]
	if !hasAsset {
		return &output.Error{
			Code:    output.CodeAPI,
			Message: fmt.Sprintf("release %s has no asset %q", latest.Tag, asset),
		}
	}
	if !hasSums {
		return &output.Error{
			Code:    output.CodeAPI,
			Message: fmt.Sprintf("release %s has no checksums.txt", latest.Tag),
		}
	}

	current := cliVersion

	if selfUpdateCheck {
		printSuccess(map[string]any{
			"current":          current,
			"latest":           latest.Tag,
			"update_available": current != latest.Tag,
			"asset":            asset,
		})
		return nil
	}

	if current == latest.Tag && !selfUpdateForce {
		printSuccess(map[string]any{
			"current": current,
			"latest":  latest.Tag,
			"updated": false,
		})
		return nil
	}

	exe, err := executableResolver()
	if err != nil {
		return err
	}

	expected, err := fetchChecksum(ctx, sumsURL, asset)
	if err != nil {
		return err
	}

	tmp, err := downloadVerified(ctx, assetURL, expected, exe)
	if err != nil {
		return err
	}

	if err := swapBinary(exe, tmp); err != nil {
		_ = os.Remove(tmp) //nolint:gosec // G703: tmp came from os.CreateTemp inside the target directory
		return err
	}

	printSuccess(map[string]any{
		"from": current,
		"to":   latest.Tag,
		"path": exe,
	})
	return nil
}

// assetNameFor returns the release asset name for the given GOOS/GOARCH and
// whether the combination is supported by self-update. Windows is not yet
// supported — replacing a running .exe requires the rename-old-to-.old dance
// and we defer that until we actually need it.
func assetNameFor(goos, goarch string) (string, bool) {
	switch goos {
	case "darwin", "linux":
		if goarch == "amd64" || goarch == "arm64" {
			return "fizzy-" + goos + "-" + goarch, true
		}
	}
	return "", false
}

// release captures the subset of the GitHub release API we use.
type release struct {
	Tag    string
	Assets map[string]string // asset name -> browser_download_url
}

func fetchLatestRelease(ctx context.Context) (*release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest",
		ghAPIBase, selfUpdateRepoOwner, selfUpdateRepoName)

	body, err := httpGetBody(ctx, url, "application/vnd.github+json")
	if err != nil {
		return nil, err
	}

	var payload struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, &output.Error{
			Code:    output.CodeAPI,
			Message: fmt.Sprintf("parsing release JSON: %v", err),
		}
	}
	if payload.TagName == "" {
		return nil, &output.Error{Code: output.CodeAPI, Message: "release has no tag"}
	}
	rel := &release{Tag: payload.TagName, Assets: make(map[string]string, len(payload.Assets))}
	for _, a := range payload.Assets {
		rel.Assets[a.Name] = a.BrowserDownloadURL
	}
	return rel, nil
}

// fetchChecksum downloads the checksums.txt and returns the hex SHA256 for
// asset. Format is the standard shasum output: "<hex>  <filename>".
func fetchChecksum(ctx context.Context, url, asset string) (string, error) {
	body, err := httpGetBody(ctx, url, "")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if fields[1] == asset {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", &output.Error{
		Code:    output.CodeAPI,
		Message: fmt.Sprintf("checksum for %q not found", asset),
	}
}

func httpGetBody(ctx context.Context, url, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	req.Header.Set("User-Agent", "fizzy-cli-self-update")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, &output.Error{
			Code:    output.CodeNetwork,
			Message: fmt.Sprintf("GET %s: %v", url, err),
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &output.Error{
			Code:       output.CodeAPI,
			HTTPStatus: resp.StatusCode,
			Message:    fmt.Sprintf("GET %s: %s", url, resp.Status),
		}
	}
	return io.ReadAll(resp.Body)
}

// downloadVerified downloads url into a temp file in the same directory as
// dest (so the final rename is atomic on one filesystem), verifies SHA256
// against expected, chmods it executable, and returns the temp path. On any
// failure, the temp file is removed by the deferred cleanup.
func downloadVerified(ctx context.Context, url, expected, dest string) (_ string, retErr error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "fizzy-cli-self-update")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", &output.Error{
			Code:    output.CodeNetwork,
			Message: fmt.Sprintf("download %s: %v", url, err),
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", &output.Error{
			Code:       output.CodeAPI,
			HTTPStatus: resp.StatusCode,
			Message:    fmt.Sprintf("download %s: %s", url, resp.Status),
		}
	}

	dir := filepath.Dir(dest)
	tmp, err := os.CreateTemp(dir, filepath.Base(dest)+".*.partial")
	if err != nil {
		return "", &output.Error{
			Code:    output.CodeAPI,
			Message: fmt.Sprintf("create temp file in %s: %v", dir, err),
			Hint:    "self-update needs write access to the directory containing the fizzy binary",
		}
	}
	tmpPath := tmp.Name()
	defer func() {
		if retErr != nil {
			_ = os.Remove(tmpPath) //nolint:gosec // G703: tmpPath came from os.CreateTemp inside dir
		}
	}()

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), resp.Body); err != nil {
		_ = tmp.Close()
		return "", &output.Error{
			Code:    output.CodeNetwork,
			Message: fmt.Sprintf("writing binary: %v", err),
		}
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	got := hex.EncodeToString(h.Sum(nil))
	if got != expected {
		return "", &output.Error{
			Code:    output.CodeAPI,
			Message: fmt.Sprintf("checksum mismatch: expected %s, got %s", expected, got),
		}
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil { //nolint:gosec // G302: 0o755 is required — we're installing an executable
		return "", err
	}
	return tmpPath, nil
}

// swapBinary atomically replaces current with newPath. On Linux and macOS
// os.Rename works even on the running executable: the kernel keeps the open
// file descriptor valid after the inode is replaced.
func swapBinary(current, newPath string) error {
	if runtime.GOOS == "windows" {
		return &output.Error{
			Code:    output.CodeAPI,
			Message: "self-update is not supported on Windows yet",
		}
	}
	if err := os.Rename(newPath, current); err != nil {
		return &output.Error{
			Code:    output.CodeAPI,
			Message: fmt.Sprintf("replacing %s: %v", current, err),
		}
	}
	return nil
}

// resolveExecutable returns the path of the currently running binary,
// following symlinks. Refuses when the resolved path is inside a git worktree
// so we don't clobber a developer's local build.
func resolveExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", &output.Error{
			Code:    output.CodeAPI,
			Message: fmt.Sprintf("resolving executable: %v", err),
		}
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", &output.Error{
			Code:    output.CodeAPI,
			Message: fmt.Sprintf("resolving symlink for %s: %v", exe, err),
		}
	}
	if insideGitWorktree(resolved) {
		return "", &output.Error{
			Code: output.CodeUsage,
			Message: fmt.Sprintf(
				"refusing to replace a build artifact inside a git worktree: %s", resolved),
			Hint: "self-update replaces the binary it runs from. " +
				"If ~/bin/fizzy is a symlink into your worktree, replace it with the released binary first: " +
				"rm ~/bin/fizzy && curl -fsSL -o ~/bin/fizzy " +
				"https://github.com/richardkmichael/fizzy-cli/releases/latest/download/fizzy-darwin-arm64 " +
				"&& chmod +x ~/bin/fizzy",
		}
	}
	return resolved, nil
}

// insideGitWorktree walks up from path looking for a .git entry.
func insideGitWorktree(path string) bool {
	dir := filepath.Dir(path)
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}
