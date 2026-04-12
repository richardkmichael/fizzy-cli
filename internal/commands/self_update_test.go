package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAssetNameFor(t *testing.T) {
	cases := []struct {
		goos, goarch string
		want         string
		supported    bool
	}{
		{"darwin", "arm64", "fizzy-darwin-arm64", true},
		{"darwin", "amd64", "fizzy-darwin-amd64", true},
		{"linux", "arm64", "fizzy-linux-arm64", true},
		{"linux", "amd64", "fizzy-linux-amd64", true},
		{"windows", "amd64", "", false},
		{"darwin", "386", "", false},
		{"plan9", "arm64", "", false},
	}
	for _, c := range cases {
		got, ok := assetNameFor(c.goos, c.goarch)
		if got != c.want || ok != c.supported {
			t.Errorf("assetNameFor(%q, %q) = (%q, %v), want (%q, %v)",
				c.goos, c.goarch, got, ok, c.want, c.supported)
		}
	}
}

// selfUpdateFixture stands up an httptest server that serves GitHub API
// release JSON and the asset + checksums downloads. The fixture rewrites
// `ghAPIBase` to point at the server and restores it on cleanup.
type selfUpdateFixture struct {
	server     *httptest.Server
	tag        string
	asset      string
	binary     []byte
	binarySHA  string
	extraFiles map[string][]byte
}

func newSelfUpdateFixture(t *testing.T, tag string) *selfUpdateFixture {
	t.Helper()
	asset, ok := assetNameFor(runtime.GOOS, runtime.GOARCH)
	if !ok {
		t.Skipf("self-update tests don't support %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	binary := []byte(fmt.Sprintf("#!/bin/sh\necho %s\n", tag))
	sum := sha256.Sum256(binary)

	f := &selfUpdateFixture{
		tag:        tag,
		asset:      asset,
		binary:     binary,
		binarySHA:  hex.EncodeToString(sum[:]),
		extraFiles: map[string][]byte{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/"+selfUpdateRepoOwner+"/"+selfUpdateRepoName+"/releases/latest",
		func(w http.ResponseWriter, r *http.Request) {
			downloadBase := "http://" + r.Host + "/download/"
			payload := map[string]any{
				"tag_name": f.tag,
				"assets": []map[string]string{
					{"name": f.asset, "browser_download_url": downloadBase + f.asset},
					{"name": "checksums.txt", "browser_download_url": downloadBase + "checksums.txt"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(payload)
		})
	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/download/")
		switch name {
		case f.asset:
			_, _ = w.Write(f.binary)
		case "checksums.txt":
			sums := fmt.Sprintf("%s  %s\n", f.binarySHA, f.asset)
			for extraName, extraBin := range f.extraFiles {
				extraSum := sha256.Sum256(extraBin)
				sums += fmt.Sprintf("%s  %s\n", hex.EncodeToString(extraSum[:]), extraName)
			}
			_, _ = w.Write([]byte(sums))
		default:
			if body, ok := f.extraFiles[name]; ok {
				_, _ = w.Write(body)
				return
			}
			http.NotFound(w, r)
		}
	})

	f.server = httptest.NewServer(mux)

	origBase := ghAPIBase
	ghAPIBase = f.server.URL
	t.Cleanup(func() {
		ghAPIBase = origBase
		f.server.Close()
	})
	return f
}

// corruptChecksum points the fixture's checksums.txt at a different hash so
// the downloaded binary fails verification.
func (f *selfUpdateFixture) corruptChecksum() {
	wrong := sha256.Sum256([]byte("not the binary"))
	f.binarySHA = hex.EncodeToString(wrong[:])
}

// installTestBinary writes a placeholder executable into a temp dir and
// returns its path. Used to simulate "the running fizzy binary" in tests.
func installTestBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fizzy")
	if err := os.WriteFile(path, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("write test binary: %v", err)
	}
	return path
}

func withExecutableOverride(t *testing.T, path string) {
	t.Helper()
	orig := executableResolver
	executableResolver = func() (string, error) { return path, nil }
	t.Cleanup(func() { executableResolver = orig })
}

func withVersion(t *testing.T, v string) {
	t.Helper()
	orig := cliVersion
	cliVersion = v
	t.Cleanup(func() { cliVersion = orig })
}

func resetSelfUpdateFlags(t *testing.T) {
	t.Helper()
	origCheck, origForce := selfUpdateCheck, selfUpdateForce
	selfUpdateCheck, selfUpdateForce = false, false
	t.Cleanup(func() {
		selfUpdateCheck, selfUpdateForce = origCheck, origForce
	})
}

func TestSelfUpdateCheckReportsNewerAvailable(t *testing.T) {
	mock := NewMockClient()
	SetTestModeWithSDK(mock)
	defer resetTest()

	fx := newSelfUpdateFixture(t, "v9.9.9-tt.1.gdeadbee")
	withVersion(t, "v3.0.3-tt.1.gabc1234")
	resetSelfUpdateFlags(t)
	selfUpdateCheck = true

	if err := selfUpdateCmd.RunE(selfUpdateCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := lastResult.Response
	if resp == nil {
		t.Fatalf("no response captured")
	}
	data, _ := resp.Data.(map[string]any)
	if data["update_available"] != true {
		t.Errorf("expected update_available=true, got %#v", data["update_available"])
	}
	if data["latest"] != fx.tag {
		t.Errorf("expected latest=%q, got %#v", fx.tag, data["latest"])
	}
	if data["asset"] != fx.asset {
		t.Errorf("expected asset=%q, got %#v", fx.asset, data["asset"])
	}
}

func TestSelfUpdateNoOpWhenAlreadyCurrent(t *testing.T) {
	mock := NewMockClient()
	SetTestModeWithSDK(mock)
	defer resetTest()

	const tag = "v3.0.3-tt.5.g0f027e0"
	newSelfUpdateFixture(t, tag)
	withVersion(t, tag)
	resetSelfUpdateFlags(t)

	// No executable override on purpose: if the no-op path accidentally
	// reaches the download step, we'd see a symlink/git-worktree error
	// pointing at the test binary — a loud failure.

	if err := selfUpdateCmd.RunE(selfUpdateCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := lastResult.Response.Data.(map[string]any)
	if data["updated"] != false {
		t.Errorf("expected updated=false, got %#v", data["updated"])
	}
	if data["current"] != tag || data["latest"] != tag {
		t.Errorf("expected current=latest=%q, got %#v", tag, data)
	}
}

func TestSelfUpdateSwapsBinary(t *testing.T) {
	mock := NewMockClient()
	SetTestModeWithSDK(mock)
	defer resetTest()

	const newTag = "v3.0.3-tt.6.gaabbccd"
	fx := newSelfUpdateFixture(t, newTag)

	target := installTestBinary(t)
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}
	if string(before) == string(fx.binary) {
		t.Fatalf("test setup: placeholder and fixture binary must differ")
	}

	withExecutableOverride(t, target)
	withVersion(t, "v3.0.3-tt.5.gffffff0")
	resetSelfUpdateFlags(t)

	if err := selfUpdateCmd.RunE(selfUpdateCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(got) != string(fx.binary) {
		t.Errorf("binary not swapped:\n  want: %q\n   got: %q", fx.binary, got)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("expected executable bit set, got mode %v", info.Mode())
	}

	// Temp dir should contain only the target — no leftover .partial file.
	entries, _ := os.ReadDir(filepath.Dir(target))
	for _, e := range entries {
		if strings.Contains(e.Name(), ".partial") {
			t.Errorf("leftover partial file: %s", e.Name())
		}
	}

	data, _ := lastResult.Response.Data.(map[string]any)
	if data["to"] != newTag {
		t.Errorf("expected to=%q, got %#v", newTag, data["to"])
	}
	if data["path"] != target {
		t.Errorf("expected path=%q, got %#v", target, data["path"])
	}
}

func TestSelfUpdateChecksumMismatchLeavesBinaryUntouched(t *testing.T) {
	mock := NewMockClient()
	SetTestModeWithSDK(mock)
	defer resetTest()

	fx := newSelfUpdateFixture(t, "v3.0.3-tt.7.gbadbad0")
	fx.corruptChecksum()

	target := installTestBinary(t)
	original, _ := os.ReadFile(target)

	withExecutableOverride(t, target)
	withVersion(t, "v3.0.3-tt.1.gcafecaf")
	resetSelfUpdateFlags(t)

	err := selfUpdateCmd.RunE(selfUpdateCmd, nil)
	if err == nil {
		t.Fatalf("expected checksum-mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("expected error to mention checksum mismatch, got: %v", err)
	}

	after, _ := os.ReadFile(target)
	if string(after) != string(original) {
		t.Errorf("target clobbered on failure:\n  want: %q\n   got: %q", original, after)
	}

	entries, _ := os.ReadDir(filepath.Dir(target))
	for _, e := range entries {
		if strings.Contains(e.Name(), ".partial") {
			t.Errorf("leftover partial file on failure: %s", e.Name())
		}
	}
}

func TestResolveExecutableRefusesGitWorktree(t *testing.T) {
	dir := t.TempDir()
	// Fake a worktree by creating a .git directory, then a bin/ with a binary.
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binDir := filepath.Join(dir, "bin")
	if err := os.Mkdir(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	exe := filepath.Join(binDir, "fizzy")
	if err := os.WriteFile(exe, []byte("x"), 0o755); err != nil {
		t.Fatalf("write exe: %v", err)
	}

	if !insideGitWorktree(exe) {
		t.Fatalf("insideGitWorktree(%q) = false, want true", exe)
	}

	// Also test the negative: a path outside any worktree.
	// A fresh t.TempDir() sibling has no .git ancestors on macOS/Linux.
	outside := filepath.Join(t.TempDir(), "fizzy")
	if err := os.WriteFile(outside, []byte("x"), 0o755); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	if insideGitWorktree(outside) {
		t.Errorf("insideGitWorktree(%q) = true, want false", outside)
	}
}

func TestFetchChecksumFindsTargetLine(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Join([]string{
			"aaaa  fizzy-darwin-amd64",
			"bbbb  fizzy-darwin-arm64",
			"cccc  fizzy-linux-amd64",
			"",
		}, "\n")))
	}))
	defer server.Close()

	got, err := fetchChecksum(context.Background(), server.URL, "fizzy-darwin-arm64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "bbbb" {
		t.Errorf("got %q, want %q", got, "bbbb")
	}

	if _, err := fetchChecksum(context.Background(), server.URL, "fizzy-freebsd-arm64"); err == nil {
		t.Errorf("expected error for missing asset, got nil")
	}
}
