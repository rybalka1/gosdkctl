package sdk

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rybalka1/gosdkctl/internal/config"
	"github.com/rybalka1/gosdkctl/internal/version"
)

type Manager struct {
	cfg config.Config
}

type InstalledVersion struct {
	Name string
	Path string
}

type MigrationResult struct {
	Version         InstalledVersion
	AlreadyMigrated bool
}

type ShellEnv struct {
	GOROOT string
	GOPATH string
	PATH   string
}

func NewManager(cfg config.Config) *Manager {
	return &Manager{cfg: cfg}
}

func (m *Manager) List() ([]string, error) {
	entries, err := os.ReadDir(m.cfg.SDKDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read sdk dir: %w", err)
	}

	versions := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !version.IsGoVersionDir(name) {
			continue
		}
		if _, err := os.Stat(filepath.Join(m.cfg.SDKDir, name, "bin", "go")); err != nil {
			continue
		}
		versions = append(versions, name)
	}

	sort.Slice(versions, func(i, j int) bool {
		return version.Compare(versions[i], versions[j]) < 0
	})

	return versions, nil
}

func (m *Manager) Current() (InstalledVersion, error) {
	if target, err := filepath.EvalSymlinks(m.cfg.DefaultLink); err == nil {
		if _, err := os.Stat(filepath.Join(target, "bin", "go")); err == nil {
			return InstalledVersion{Name: filepath.Base(target), Path: target}, nil
		}
	}

	if _, err := os.Stat(filepath.Join(m.cfg.LocalGoDir, "bin", "go")); err == nil {
		return InstalledVersion{Name: filepath.Base(m.cfg.LocalGoDir), Path: m.cfg.LocalGoDir}, nil
	}

	versions, err := m.List()
	if err != nil {
		return InstalledVersion{}, err
	}
	if len(versions) == 0 {
		return InstalledVersion{}, fmt.Errorf("no installed Go SDKs found in %s", m.cfg.SDKDir)
	}

	name := versions[len(versions)-1]
	return InstalledVersion{Name: name, Path: filepath.Join(m.cfg.SDKDir, name)}, nil
}

func (m *Manager) Switch(name string) (InstalledVersion, error) {
	if name == "default" {
		return m.Current()
	}
	if !version.IsGoVersionDir(name) {
		return InstalledVersion{}, fmt.Errorf("invalid Go SDK name %q", name)
	}

	path := filepath.Join(m.cfg.SDKDir, name)
	if _, err := os.Stat(filepath.Join(path, "bin", "go")); err != nil {
		return InstalledVersion{}, fmt.Errorf("%s is not installed in %s", name, m.cfg.SDKDir)
	}
	if err := m.setDefaultLink(path); err != nil {
		return InstalledVersion{}, err
	}
	return InstalledVersion{Name: name, Path: path}, nil
}

func (m *Manager) InstallArchive(ctx context.Context, archivePath string) (InstalledVersion, bool, error) {
	if err := os.MkdirAll(m.cfg.SDKDir, 0o755); err != nil {
		return InstalledVersion{}, false, fmt.Errorf("create sdk dir: %w", err)
	}

	name, err := readArchiveSDKName(ctx, archivePath)
	if err != nil {
		return InstalledVersion{}, false, err
	}

	dest := filepath.Join(m.cfg.SDKDir, name)
	if _, err := os.Stat(dest); err == nil {
		if _, err := validateSDK(dest); err != nil {
			return InstalledVersion{}, false, err
		}
		if err := m.setDefaultLink(dest); err != nil {
			return InstalledVersion{}, false, err
		}
		return InstalledVersion{Name: name, Path: dest}, true, nil
	} else if err != nil && !os.IsNotExist(err) {
		return InstalledVersion{}, false, fmt.Errorf("inspect destination: %w", err)
	}

	tmp, err := os.MkdirTemp(m.cfg.SDKDir, ".gosdkctl-install-*")
	if err != nil {
		return InstalledVersion{}, false, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	if err := extractGoArchive(ctx, archivePath, tmp); err != nil {
		return InstalledVersion{}, false, err
	}

	extracted := filepath.Join(tmp, "go")
	extractedName, err := validateSDK(extracted)
	if err != nil {
		return InstalledVersion{}, false, err
	}
	if extractedName != name {
		return InstalledVersion{}, false, fmt.Errorf("archive version changed while extracting: %s -> %s", name, extractedName)
	}
	if err := os.Rename(extracted, dest); err != nil {
		return InstalledVersion{}, false, fmt.Errorf("move sdk into place: %w", err)
	}

	if err := m.setDefaultLink(dest); err != nil {
		return InstalledVersion{}, false, err
	}
	return InstalledVersion{Name: name, Path: dest}, false, nil
}

func (m *Manager) MigrateLocal() (MigrationResult, error) {
	name, err := validateSDK(m.cfg.LocalGoDir)
	if err != nil {
		return MigrationResult{}, fmt.Errorf("legacy ~/.local/go is not a valid Go SDK: %w", err)
	}
	if err := os.MkdirAll(m.cfg.SDKDir, 0o755); err != nil {
		return MigrationResult{}, fmt.Errorf("create sdk dir: %w", err)
	}

	dest := filepath.Join(m.cfg.SDKDir, name)
	result := MigrationResult{Version: InstalledVersion{Name: name, Path: dest}}
	if _, err := os.Stat(dest); err == nil {
		result.AlreadyMigrated = true
	} else if os.IsNotExist(err) {
		if err := os.Rename(m.cfg.LocalGoDir, dest); err != nil {
			return MigrationResult{}, fmt.Errorf("move ~/.local/go into sdk dir: %w", err)
		}
	} else {
		return MigrationResult{}, fmt.Errorf("inspect destination: %w", err)
	}

	if err := m.setDefaultLink(dest); err != nil {
		return MigrationResult{}, err
	}
	return result, nil
}

func (m *Manager) Doctor() (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Home: %s\n", m.cfg.Home)
	fmt.Fprintf(&b, "SDK dir: %s\n", m.cfg.SDKDir)
	fmt.Fprintf(&b, "Default link: %s\n", m.cfg.DefaultLink)

	if info, err := os.Lstat(m.cfg.DefaultLink); os.IsNotExist(err) {
		fmt.Fprintln(&b, "Default target: no current default")
	} else if err != nil {
		fmt.Fprintf(&b, "Default target: inspect error: %v\n", err)
	} else if info.Mode()&os.ModeSymlink == 0 {
		fmt.Fprintln(&b, "Default target: go-current exists but is not a symlink")
	} else if target, err := filepath.EvalSymlinks(m.cfg.DefaultLink); err == nil {
		fmt.Fprintf(&b, "Default target: %s\n", target)
	} else {
		fmt.Fprintf(&b, "Default target: broken symlink: %v\n", err)
	}

	fmt.Fprintf(&b, "GOROOT: %s\n", os.Getenv("GOROOT"))
	fmt.Fprintf(&b, "GOPATH: %s\n", m.goPath())
	fmt.Fprintf(&b, "PATH: %s\n", os.Getenv("PATH"))

	if goPath, err := findInPath("go"); err == nil {
		fmt.Fprintf(&b, "go in PATH: %s\n", goPath)
	} else {
		fmt.Fprintf(&b, "go in PATH: %v\n", err)
	}

	if _, err := os.Stat(filepath.Join(m.cfg.LocalGoDir, "bin", "go")); err == nil {
		fmt.Fprintf(&b, "legacy ~/.local/go: present at %s\n", m.cfg.LocalGoDir)
	} else {
		fmt.Fprintln(&b, "legacy ~/.local/go: not present")
	}

	versions, err := m.List()
	if err != nil {
		return "", err
	}
	fmt.Fprintln(&b, "Installed versions:")
	if len(versions) == 0 {
		fmt.Fprintln(&b, "  (none)")
	} else {
		for _, name := range versions {
			fmt.Fprintf(&b, "  %s\n", name)
		}
	}

	return b.String(), nil
}

func (m *Manager) Env(target string) (ShellEnv, error) {
	var selected InstalledVersion
	if target == "" || target == "default" {
		var err error
		selected, err = m.Current()
		if err != nil {
			return ShellEnv{}, err
		}
	} else if version.IsGoVersionDir(target) {
		path := filepath.Join(m.cfg.SDKDir, target)
		if _, err := validateSDK(path); err != nil {
			return ShellEnv{}, fmt.Errorf("%s is not installed in %s", target, m.cfg.SDKDir)
		}
		selected = InstalledVersion{Name: target, Path: path}
	} else {
		path, err := filepath.Abs(target)
		if err != nil {
			return ShellEnv{}, fmt.Errorf("resolve SDK path: %w", err)
		}
		name, err := validateSDK(path)
		if err != nil {
			return ShellEnv{}, fmt.Errorf("%s is not a valid Go SDK: %w", target, err)
		}
		selected = InstalledVersion{Name: name, Path: path}
	}
	return ShellEnv{GOROOT: selected.Path, GOPATH: m.goPath(), PATH: buildPath(selected.Path, m.goPath(), m.cfg.SDKDir)}, nil
}

func (m *Manager) setDefaultLink(target string) error {
	if err := os.MkdirAll(m.cfg.SDKDir, 0o755); err != nil {
		return fmt.Errorf("create sdk dir: %w", err)
	}
	if info, err := os.Lstat(m.cfg.DefaultLink); err == nil && info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%s exists and is not a symlink", m.cfg.DefaultLink)
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect default symlink: %w", err)
	}

	linkTmp := filepath.Join(m.cfg.SDKDir, fmt.Sprintf(".go-current-%d", time.Now().UnixNano()))
	if err := os.Symlink(target, linkTmp); err != nil {
		return fmt.Errorf("create default symlink: %w", err)
	}
	if err := os.Rename(linkTmp, m.cfg.DefaultLink); err != nil {
		_ = os.Remove(linkTmp)
		return fmt.Errorf("update default symlink: %w", err)
	}
	return nil
}

func (m *Manager) goPath() string {
	if value := os.Getenv("GOPATH"); value != "" {
		return value
	}
	return filepath.Join(m.cfg.Home, "go")
}

func readArchiveSDKName(ctx context.Context, archivePath string) (string, error) {
	tr, closeFn, err := openGoArchive(archivePath)
	if err != nil {
		return "", err
	}
	defer closeFn()

	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		header, err := tr.Next()
		if err == io.EOF {
			return "", fmt.Errorf("archive does not contain go/VERSION")
		}
		if err != nil {
			return "", fmt.Errorf("read tar archive: %w", err)
		}
		name, ok, err := cleanArchiveName(header.Name)
		if err != nil {
			return "", err
		}
		if !ok || name != "go/VERSION" {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return "", fmt.Errorf("read Go VERSION from archive: %w", err)
		}
		fields := strings.Fields(string(data))
		if len(fields) == 0 || !version.IsGoVersionDir(fields[0]) {
			return "", fmt.Errorf("invalid Go VERSION file in archive")
		}
		return fields[0], nil
	}
}

func extractGoArchive(ctx context.Context, archivePath, dest string) error {
	tr, closeFn, err := openGoArchive(archivePath)
	if err != nil {
		return err
	}
	defer closeFn()

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar archive: %w", err)
		}
		name, ok, err := cleanArchiveName(header.Name)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		target, err := secureJoin(dest, name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := ensureNoSymlinkInPath(dest, filepath.Dir(target)); err != nil {
				return err
			}
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("create directory from archive: %w", err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := ensureWritableTarget(dest, target); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file from archive: %w", err)
			}
			_, copyErr := io.Copy(out, tr)
			closeErr := out.Close()
			if copyErr != nil {
				return fmt.Errorf("extract file from archive: %w", copyErr)
			}
			if closeErr != nil {
				return fmt.Errorf("close extracted file: %w", closeErr)
			}
		case tar.TypeSymlink:
			linkName, err := cleanArchiveLink(header.Linkname)
			if err != nil {
				return fmt.Errorf("unsafe symlink %q -> %q: %w", header.Name, header.Linkname, err)
			}
			if err := ensureWritableTarget(dest, target); err != nil {
				return err
			}
			if err := os.Symlink(linkName, target); err != nil {
				return fmt.Errorf("create symlink from archive: %w", err)
			}
		case tar.TypeLink:
			linkName, ok, err := cleanArchiveName(header.Linkname)
			if err != nil {
				return fmt.Errorf("unsafe hardlink %q -> %q: %w", header.Name, header.Linkname, err)
			}
			if !ok {
				return fmt.Errorf("hardlink %q points outside go archive root", header.Name)
			}
			source, err := secureJoin(dest, linkName)
			if err != nil {
				return err
			}
			if err := ensureWritableTarget(dest, target); err != nil {
				return err
			}
			if err := os.Link(source, target); err != nil {
				return fmt.Errorf("create hardlink from archive: %w", err)
			}
		}
	}
	return nil
}

func openGoArchive(archivePath string) (*tar.Reader, func(), error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, nil, fmt.Errorf("open archive: %w", err)
	}
	gz, err := gzip.NewReader(file)
	if err != nil {
		_ = file.Close()
		return nil, nil, fmt.Errorf("read gzip archive: %w", err)
	}
	return tar.NewReader(gz), func() {
		_ = gz.Close()
		_ = file.Close()
	}, nil
}

func validateSDK(root string) (string, error) {
	versionFile := filepath.Join(root, "VERSION")
	data, err := os.ReadFile(versionFile)
	if err != nil {
		return "", fmt.Errorf("read Go VERSION file: %w", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 || !version.IsGoVersionDir(fields[0]) {
		return "", fmt.Errorf("invalid Go VERSION file")
	}
	if _, err := os.Stat(filepath.Join(root, "bin", "go")); err != nil {
		return "", fmt.Errorf("SDK does not contain bin/go: %w", err)
	}
	return fields[0], nil
}

func cleanArchiveName(name string) (string, bool, error) {
	if name == "" || filepath.IsAbs(name) {
		return "", false, fmt.Errorf("unsafe archive path %q", name)
	}
	clean := filepath.Clean(name)
	if clean == "." {
		return "", false, fmt.Errorf("unsafe archive path %q", name)
	}
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		if part == ".." {
			return "", false, fmt.Errorf("unsafe archive path %q", name)
		}
	}
	if clean != "go" && !strings.HasPrefix(clean, "go"+string(filepath.Separator)) {
		return "", false, nil
	}
	return clean, true, nil
}

func cleanArchiveLink(linkName string) (string, error) {
	if linkName == "" || filepath.IsAbs(linkName) {
		return "", fmt.Errorf("absolute or empty link target")
	}
	clean := filepath.Clean(linkName)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("link target escapes archive root")
	}
	return clean, nil
}

func secureJoin(root, name string) (string, error) {
	target := filepath.Join(root, name)
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", fmt.Errorf("resolve archive path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("archive path escapes destination %q", name)
	}
	return target, nil
}

func ensureWritableTarget(root, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	if err := ensureNoSymlinkInPath(root, filepath.Dir(target)); err != nil {
		return err
	}
	if info, err := os.Lstat(target); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to overwrite symlink %s", target)
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect archive target: %w", err)
	}
	return nil
}

func ensureNoSymlinkInPath(root, target string) error {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return fmt.Errorf("resolve path below destination: %w", err)
	}
	if rel == "." {
		return nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("path escapes destination %s", target)
	}
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect archive path: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive path crosses symlink %s", current)
		}
	}
	return nil
}

func buildPath(goRoot, goPath, sdkDir string) string {
	blocked := []string{filepath.Join(sdkDir, "go-current", "bin")}
	entries, _ := os.ReadDir(sdkDir)
	for _, entry := range entries {
		if entry.IsDir() && version.IsGoVersionDir(entry.Name()) {
			blocked = append(blocked, filepath.Join(sdkDir, entry.Name(), "bin"))
		}
	}

	parts := []string{filepath.Join(goRoot, "bin"), filepath.Join(goPath, "bin"), sdkDir}
	seen := map[string]bool{}
	for _, part := range parts {
		if part != "" {
			seen[part] = true
		}
	}
	for _, part := range filepath.SplitList(os.Getenv("PATH")) {
		if part == "" || seen[part] || containsPath(blocked, part) {
			continue
		}
		parts = append(parts, part)
		seen[part] = true
	}
	return strings.Join(parts, string(os.PathListSeparator))
}

func containsPath(paths []string, target string) bool {
	for _, path := range paths {
		if path == target {
			return true
		}
	}
	return false
}

func findInPath(name string) (string, error) {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s not found", name)
}
