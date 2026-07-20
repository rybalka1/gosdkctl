package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rybalka1/gosdkctl/internal/config"
	"github.com/rybalka1/gosdkctl/internal/sdk"
)

func TestRunHelp(t *testing.T) {
	t.Parallel()

	command, _, stdout, _ := newTestCommand(t, "")
	if err := command.Run(context.Background(), []string{"-help"}); err != nil {
		t.Fatalf("Run(-help) error = %v", err)
	}
	if !strings.Contains(stdout.String(), "gosdkctl migrate-local") {
		t.Fatalf("help output does not include migrate-local:\n%s", stdout.String())
	}
}

func TestRunChoose(t *testing.T) {
	t.Parallel()

	command, manager, stdout, _ := newTestCommand(t, "2\n")
	makeSDK(t, manager.SDKDir, "go1.24.2")
	makeSDK(t, manager.SDKDir, "go1.26.1")

	if err := command.Run(context.Background(), []string{"choose"}); err != nil {
		t.Fatalf("Run(choose) error = %v", err)
	}
	if !strings.Contains(stdout.String(), "go-current ->") {
		t.Fatalf("choose output = %q", stdout.String())
	}
	target, err := filepath.EvalSymlinks(manager.DefaultLink)
	if err != nil {
		t.Fatalf("EvalSymlinks() error = %v", err)
	}
	if filepath.Base(target) != "go1.26.1" {
		t.Fatalf("go-current target = %q", target)
	}
}

func TestRunEnv(t *testing.T) {
	t.Parallel()

	command, manager, stdout, _ := newTestCommand(t, "")
	makeSDK(t, manager.SDKDir, "go1.26.1")

	if err := command.Run(context.Background(), []string{"env", "go1.26.1"}); err != nil {
		t.Fatalf("Run(env) error = %v", err)
	}
	if !strings.Contains(stdout.String(), "export GOROOT=") {
		t.Fatalf("env output = %q", stdout.String())
	}
}

func newTestCommand(t *testing.T, input string) (*Command, config.Config, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	home := t.TempDir()
	cfg := config.Config{
		Home:        home,
		SDKDir:      filepath.Join(home, "sdk"),
		DefaultLink: filepath.Join(home, "sdk", "go-current"),
		LocalGoDir:  filepath.Join(home, ".local", "go"),
	}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	return New(sdk.NewManager(cfg), strings.NewReader(input), stdout, stderr), cfg, stdout, stderr
}

func makeSDK(t *testing.T, sdkDir, name string) {
	t.Helper()
	root := filepath.Join(sdkDir, name)
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "go"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte(name+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
