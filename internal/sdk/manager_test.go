package sdk

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rybalka1/gosdkctl/internal/config"
)

func TestListSwitchAndEnv(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	makeSDK(t, manager.cfg.SDKDir, "go1.24.2")
	makeSDK(t, manager.cfg.SDKDir, "go1.26.1")

	versions, err := manager.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got, want := strings.Join(versions, ","), "go1.24.2,go1.26.1"; got != want {
		t.Fatalf("List() = %q, want %q", got, want)
	}

	selected, err := manager.Switch("go1.24.2")
	if err != nil {
		t.Fatalf("Switch() error = %v", err)
	}
	if selected.Name != "go1.24.2" {
		t.Fatalf("Switch() selected %q", selected.Name)
	}

	current, err := manager.Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if current.Name != "go1.24.2" {
		t.Fatalf("Current() = %q, want go1.24.2", current.Name)
	}

	env, err := manager.Env("go1.26.1")
	if err != nil {
		t.Fatalf("Env() error = %v", err)
	}
	if !strings.HasSuffix(env.GOROOT, filepath.Join("sdk", "go1.26.1")) {
		t.Fatalf("Env().GOROOT = %q", env.GOROOT)
	}
	if !strings.HasPrefix(env.PATH, filepath.Join(env.GOROOT, "bin")) {
		t.Fatalf("Env().PATH = %q", env.PATH)
	}
}

func TestInstallArchive(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	archive := filepath.Join(t.TempDir(), "go1.27.0.linux-amd64.tar.gz")
	writeArchive(t, archive, "go1.27.0")

	installed, existed, err := manager.InstallArchive(context.Background(), archive)
	if err != nil {
		t.Fatalf("InstallArchive() error = %v", err)
	}
	if existed {
		t.Fatal("InstallArchive() reported existing SDK on first install")
	}
	if installed.Name != "go1.27.0" {
		t.Fatalf("installed.Name = %q", installed.Name)
	}
	if _, err := os.Stat(filepath.Join(manager.cfg.SDKDir, "go1.27.0", "bin", "go")); err != nil {
		t.Fatalf("installed go binary missing: %v", err)
	}

	installed, existed, err = manager.InstallArchive(context.Background(), archive)
	if err != nil {
		t.Fatalf("second InstallArchive() error = %v", err)
	}
	if !existed || installed.Name != "go1.27.0" {
		t.Fatalf("second install = (%+v, %v), want existing go1.27.0", installed, existed)
	}
}

func TestInstallDownloadLatest(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	archivePath := filepath.Join(t.TempDir(), "go1.30.0.tar.gz")
	writeArchive(t, archivePath, "go1.30.0")
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	sum := sha256.Sum256(archiveData)
	filename := fmt.Sprintf("go1.30.0.%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dl/":
			fmt.Fprintf(w, `[
				{"version":"go1.29.0","stable":true,"files":[{"filename":"old.tar.gz","os":%q,"arch":%q,"version":"go1.29.0","sha256":"old","kind":"archive"}]},
				{"version":"go1.30.0","stable":true,"files":[{"filename":%q,"os":%q,"arch":%q,"version":"go1.30.0","sha256":%q,"kind":"archive"}]}
			]`, runtime.GOOS, runtime.GOARCH, filename, runtime.GOOS, runtime.GOARCH, hex.EncodeToString(sum[:]))
		case "/dl/" + filename:
			_, _ = w.Write(archiveData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	manager.goDownloadAPI = server.URL + "/dl/?mode=json&include=all"
	manager.goDownloadBase = server.URL + "/dl/"

	result, err := manager.InstallDownload(context.Background(), "latest")
	if err != nil {
		t.Fatalf("InstallDownload() error = %v", err)
	}
	if result.Version.Name != "go1.30.0" || result.File != filename {
		t.Fatalf("InstallDownload() = %+v", result)
	}
}

func TestInstallDownloadSpecificVersion(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	archivePath := filepath.Join(t.TempDir(), "go1.29.0.tar.gz")
	writeArchive(t, archivePath, "go1.29.0")
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	sum := sha256.Sum256(archiveData)
	filename := fmt.Sprintf("go1.29.0.%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dl/":
			fmt.Fprintf(w, `[{"version":"go1.29.0","stable":true,"files":[{"filename":%q,"os":%q,"arch":%q,"version":"go1.29.0","sha256":%q,"kind":"archive"}]}]`, filename, runtime.GOOS, runtime.GOARCH, hex.EncodeToString(sum[:]))
		case "/dl/" + filename:
			_, _ = w.Write(archiveData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	manager.goDownloadAPI = server.URL + "/dl/?mode=json&include=all"
	manager.goDownloadBase = server.URL + "/dl/"

	result, err := manager.InstallDownload(context.Background(), "go1.29.0")
	if err != nil {
		t.Fatalf("InstallDownload(go1.29.0) error = %v", err)
	}
	if result.Version.Name != "go1.29.0" || result.File != filename {
		t.Fatalf("InstallDownload(go1.29.0) = %+v", result)
	}
}

func TestInstallDownloadRejectsBadChecksum(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	filename := fmt.Sprintf("go1.30.0.%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dl/":
			fmt.Fprintf(w, `[{"version":"go1.30.0","stable":true,"files":[{"filename":%q,"os":%q,"arch":%q,"version":"go1.30.0","sha256":"deadbeef","kind":"archive"}]}]`, filename, runtime.GOOS, runtime.GOARCH)
		case "/dl/" + filename:
			_, _ = w.Write([]byte("bad archive"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	manager.goDownloadAPI = server.URL + "/dl/?mode=json&include=all"
	manager.goDownloadBase = server.URL + "/dl/"

	if _, err := manager.InstallDownload(context.Background(), "latest"); err == nil {
		t.Fatal("InstallDownload() accepted bad checksum")
	}
}

func TestInstallDownloadRejectsLargeMetadata(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", metadataLimit+1)))
	}))
	defer server.Close()

	manager.goDownloadAPI = server.URL

	if _, err := manager.InstallDownload(context.Background(), "latest"); err == nil {
		t.Fatal("InstallDownload() accepted oversized metadata")
	}
}

func TestSelfInstall(t *testing.T) {
	manager := newTestManager(t)
	t.Setenv("PATH", filepath.Join(manager.cfg.Home, ".local", "bin"))
	source := filepath.Join(t.TempDir(), "gosdkctl-source")
	if err := os.WriteFile(source, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := manager.SelfInstall(source)
	if err != nil {
		t.Fatalf("SelfInstall() error = %v", err)
	}
	if _, err := os.Stat(result.BinaryPath); err != nil {
		t.Fatalf("installed binary missing: %v", err)
	}
	target, err := os.Readlink(result.AliasPath)
	if err != nil {
		t.Fatalf("Readlink() error = %v", err)
	}
	if target != result.BinaryPath {
		t.Fatalf("alias target = %q, want %q", target, result.BinaryPath)
	}
	if !result.OnPath {
		t.Fatal("SelfInstall() did not report ~/.local/bin on PATH")
	}

	if err := os.WriteFile(result.BinaryPath, []byte("old"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := manager.SelfInstall(source); err != nil {
		t.Fatalf("second SelfInstall() error = %v", err)
	}
	data, err := os.ReadFile(result.BinaryPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "#!/bin/sh\n" {
		t.Fatalf("installed binary was not replaced: %q", string(data))
	}
}

func TestInstallArchiveRejectsExistingBrokenDestination(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	archive := filepath.Join(t.TempDir(), "go1.27.0.linux-amd64.tar.gz")
	writeArchive(t, archive, "go1.27.0")
	if err := os.MkdirAll(filepath.Join(manager.cfg.SDKDir, "go1.27.0"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if _, _, err := manager.InstallArchive(context.Background(), archive); err == nil {
		t.Fatal("InstallArchive() accepted existing broken destination")
	}
}

func TestInstallArchiveRejectsVersionRewrite(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	archive := filepath.Join(t.TempDir(), "rewrite.tar.gz")
	writeArchiveEntries(t, archive, []tarEntry{
		{name: "go/VERSION", mode: 0o644, body: "go1.27.0\n"},
		{name: "go/bin/go", mode: 0o755, body: "#!/bin/sh\n"},
		{name: "go/VERSION", mode: 0o644, body: "go1.28.0\n"},
	})

	if _, _, err := manager.InstallArchive(context.Background(), archive); err == nil {
		t.Fatal("InstallArchive() accepted archive with rewritten VERSION")
	}
}

func TestInstallArchiveRejectsLargeVersionFile(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	archive := filepath.Join(t.TempDir(), "large-version.tar.gz")
	writeArchiveEntries(t, archive, []tarEntry{
		{name: "go/VERSION", mode: 0o644, body: strings.Repeat("x", versionFileLimit+1)},
		{name: "go/bin/go", mode: 0o755, body: "#!/bin/sh\n"},
	})

	if _, _, err := manager.InstallArchive(context.Background(), archive); err == nil {
		t.Fatal("InstallArchive() accepted oversized VERSION")
	}
}

func TestInstallArchiveRejectsUnsafeSymlink(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	archive := filepath.Join(t.TempDir(), "evil.tar.gz")
	writeArchiveEntries(t, archive, []tarEntry{
		{name: "go/VERSION", mode: 0o644, body: "go1.27.0\n"},
		{name: "go/bin/go", mode: 0o755, body: "#!/bin/sh\n"},
		{name: "go/pkg", mode: 0o777, typeflag: tar.TypeSymlink, linkname: "../../outside"},
	})

	if _, _, err := manager.InstallArchive(context.Background(), archive); err == nil {
		t.Fatal("InstallArchive() accepted unsafe symlink")
	}
}

func TestInstallArchiveRejectsWriteThroughSymlink(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	archive := filepath.Join(t.TempDir(), "evil.tar.gz")
	writeArchiveEntries(t, archive, []tarEntry{
		{name: "go/VERSION", mode: 0o644, body: "go1.27.0\n"},
		{name: "go/bin/go", mode: 0o755, body: "#!/bin/sh\n"},
		{name: "go/pkg", mode: 0o777, typeflag: tar.TypeSymlink, linkname: "src"},
		{name: "go/pkg/escape", mode: 0o644, body: "bad"},
	})

	if _, _, err := manager.InstallArchive(context.Background(), archive); err == nil {
		t.Fatal("InstallArchive() wrote through a symlink")
	}
}

func TestInstallArchiveHandlesHardlink(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	archive := filepath.Join(t.TempDir(), "go1.27.0.tar.gz")
	writeArchiveEntries(t, archive, []tarEntry{
		{name: "go/VERSION", mode: 0o644, body: "go1.27.0\n"},
		{name: "go/bin/go", mode: 0o755, body: "#!/bin/sh\n"},
		{name: "go/bin/gofmt", mode: 0o755, typeflag: tar.TypeLink, linkname: "go/bin/go"},
	})

	if _, _, err := manager.InstallArchive(context.Background(), archive); err != nil {
		t.Fatalf("InstallArchive() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(manager.cfg.SDKDir, "go1.27.0", "bin", "gofmt")); err != nil {
		t.Fatalf("hardlink target missing: %v", err)
	}
}

func TestSwitchRejectsNonSymlinkCurrent(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	makeSDK(t, manager.cfg.SDKDir, "go1.26.1")
	if err := os.MkdirAll(manager.cfg.DefaultLink, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	if _, err := manager.Switch("go1.26.1"); err == nil {
		t.Fatal("Switch() replaced non-symlink go-current")
	}
}

func TestEnvAcceptsSDKPath(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	path := filepath.Join(t.TempDir(), "custom-go")
	makeSDK(t, filepath.Dir(path), filepath.Base(path))
	if err := os.WriteFile(filepath.Join(path, "VERSION"), []byte("go1.28.0\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	env, err := manager.Env(path)
	if err != nil {
		t.Fatalf("Env(path) error = %v", err)
	}
	if env.GOROOT != path {
		t.Fatalf("Env(path).GOROOT = %q, want %q", env.GOROOT, path)
	}
}

func TestMigrateLocal(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	makeSDK(t, filepath.Dir(manager.cfg.LocalGoDir), filepath.Base(manager.cfg.LocalGoDir))
	if err := os.WriteFile(filepath.Join(manager.cfg.LocalGoDir, "VERSION"), []byte("go1.29.0\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := manager.MigrateLocal()
	if err != nil {
		t.Fatalf("MigrateLocal() error = %v", err)
	}
	if result.Version.Name != "go1.29.0" {
		t.Fatalf("MigrateLocal().Version.Name = %q", result.Version.Name)
	}
	if _, err := os.Stat(filepath.Join(manager.cfg.SDKDir, "go1.29.0", "bin", "go")); err != nil {
		t.Fatalf("migrated SDK missing: %v", err)
	}
}

func TestInitShellWritesAndReplacesManagedBlock(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	zshrc := filepath.Join(manager.cfg.Home, ".zshrc")
	initial := strings.Join([]string{
		"# user config",
		managedBlockStart,
		"old",
		managedBlockEnd,
		"usego() {",
		"  echo \"}\"",
		"  # { comment }",
		"  echo old",
		"}",
		"",
		"alias ll='ls -la'",
		"",
	}, "\n")
	if err := os.WriteFile(zshrc, []byte(initial), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := manager.InitShell("zsh")
	if err != nil {
		t.Fatalf("InitShell() error = %v", err)
	}
	if !result.Changed {
		t.Fatal("InitShell() did not report changed config")
	}
	data, err := os.ReadFile(zshrc)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	if !strings.HasPrefix(got, managedBlockStart) {
		t.Fatalf("managed block should be at the top:\n%s", got)
	}
	if strings.Contains(got, "\nold\n") {
		t.Fatalf("managed block was not replaced:\n%s", got)
	}
	if strings.Contains(got, "echo old") {
		t.Fatalf("legacy usego function was not removed:\n%s", got)
	}
	if !strings.Contains(got, `eval "$(gosdkctl env "${1:-default}")"`) {
		t.Fatalf("managed block does not contain usego helper:\n%s", got)
	}
	if !strings.Contains(got, "$HOME/.local/go") || !strings.Contains(got, "find \"$HOME/sdk\"") {
		t.Fatalf("managed block does not contain Go fallback logic:\n%s", got)
	}
	if !strings.Contains(got, "alias ll='ls -la'") {
		t.Fatalf("user config was not preserved:\n%s", got)
	}

	result, err = manager.InitShell("zsh")
	if err != nil {
		t.Fatalf("second InitShell() error = %v", err)
	}
	if result.Changed {
		t.Fatal("second InitShell() should be idempotent")
	}
}

func TestInitShellRejectsIncompleteManagedBlock(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	zshrc := filepath.Join(manager.cfg.Home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte(managedBlockStart+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := manager.InitShell("zsh"); err == nil {
		t.Fatal("InitShell() accepted incomplete managed block")
	}
}

func TestInitShellBash(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	result, err := manager.InitShell("bash")
	if err != nil {
		t.Fatalf("InitShell(bash) error = %v", err)
	}
	if result.Path != filepath.Join(manager.cfg.Home, ".bashrc") {
		t.Fatalf("InitShell(bash).Path = %q", result.Path)
	}
}

func TestBuildPathFiltersOnlyGoSDKBins(t *testing.T) {
	home := t.TempDir()
	sdkDir := filepath.Join(home, "sdk")
	makeSDK(t, sdkDir, "go1.26.5")
	makeSDK(t, sdkDir, "go1.25.1")
	if err := os.MkdirAll(filepath.Join(sdkDir, "gosdkctl-tool", "bin"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	keep := filepath.Join(sdkDir, "gosdkctl-tool", "bin")
	t.Setenv("PATH", strings.Join([]string{
		filepath.Join(sdkDir, "go-current", "bin"),
		filepath.Join(sdkDir, "go1.25.1", "bin"),
		keep,
		"/usr/bin",
	}, string(os.PathListSeparator)))

	got := buildPath(filepath.Join(sdkDir, "go1.26.5"), filepath.Join(home, "go"), sdkDir)
	if strings.Contains(got, filepath.Join(sdkDir, "go-current", "bin")) {
		t.Fatalf("PATH still contains go-current/bin: %q", got)
	}
	if strings.Contains(got, filepath.Join(sdkDir, "go1.25.1", "bin")) {
		t.Fatalf("PATH still contains stale go version bin: %q", got)
	}
	if !strings.Contains(got, keep) {
		t.Fatalf("PATH dropped non-version go* bin: %q", got)
	}
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	home := t.TempDir()
	return NewManager(config.Config{
		Home:        home,
		SDKDir:      filepath.Join(home, "sdk"),
		DefaultLink: filepath.Join(home, "sdk", "go-current"),
		LocalGoDir:  filepath.Join(home, ".local", "go"),
	})
}

func makeSDK(t *testing.T, sdkDir, name string) {
	t.Helper()
	bin := filepath.Join(sdkDir, name, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sdkDir, name, "VERSION"), []byte(name+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func writeArchive(t *testing.T, path, version string) {
	t.Helper()
	writeArchiveEntries(t, path, []tarEntry{
		{name: "go/VERSION", mode: 0o644, body: version + "\n"},
		{name: "go/bin/go", mode: 0o755, body: "#!/bin/sh\n"},
	})
}

type tarEntry struct {
	name     string
	mode     int64
	body     string
	typeflag byte
	linkname string
}

func writeArchiveEntries(t *testing.T, path string, entries []tarEntry) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer file.Close()

	gz := gzip.NewWriter(file)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	for _, entry := range entries {
		typeflag := entry.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		header := &tar.Header{
			Name:     entry.name,
			Mode:     entry.mode,
			Size:     int64(len(entry.body)),
			Typeflag: typeflag,
			Linkname: entry.linkname,
		}
		if typeflag == tar.TypeSymlink || typeflag == tar.TypeLink {
			header.Size = 0
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("WriteHeader() error = %v", err)
		}
		if typeflag == tar.TypeSymlink || typeflag == tar.TypeLink {
			continue
		}
		if _, err := tw.Write([]byte(entry.body)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}
}
