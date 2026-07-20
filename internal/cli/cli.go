package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/rybalka1/gosdkctl/internal/sdk"
)

type Command struct {
	manager *sdk.Manager
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
}

func New(manager *sdk.Manager, stdin io.Reader, stdout, stderr io.Writer) *Command {
	return &Command{
		manager: manager,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
	}
}

func (c *Command) Run(ctx context.Context, args []string) error {
	_ = ctx

	if len(args) == 0 {
		return c.status()
	}

	switch args[0] {
	case "status":
		return c.status()
	case "list":
		return c.list()
	case "current":
		return c.current()
	case "switch":
		if len(args) < 2 {
			return c.choose()
		}
		return c.switchDefault(args[1])
	case "choose":
		return c.choose()
	case "install":
		if len(args) != 2 {
			return fmt.Errorf("usage: gosdkctl install <archive.tar.gz|version|latest>")
		}
		return c.install(ctx, args[1])
	case "self":
		if len(args) != 2 || args[1] != "install" {
			return fmt.Errorf("usage: gosdkctl self install")
		}
		return c.selfInstall()
	case "migrate-local":
		return c.migrateLocal()
	case "init":
		shell := "auto"
		if len(args) > 1 {
			shell = args[1]
		}
		return c.initShell(shell)
	case "doctor":
		return c.doctor()
	case "env":
		target := "default"
		if len(args) > 1 {
			target = args[1]
		}
		return c.env(target)
	case "help", "-h", "-help", "--help":
		c.help()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (c *Command) status() error {
	current, err := c.manager.Current()
	if err != nil {
		fmt.Fprintln(c.stdout, "Current default: (none)")
		fmt.Fprintf(c.stderr, "warning: %v\n", err)
	} else {
		fmt.Fprintln(c.stdout, "Current default:")
		fmt.Fprintf(c.stdout, "%s -> %s\n", current.Name, current.Path)
	}

	versions, err := c.manager.List()
	if err != nil {
		return err
	}

	fmt.Fprintln(c.stdout)
	fmt.Fprintln(c.stdout, "Installed versions:")
	if len(versions) == 0 {
		fmt.Fprintln(c.stdout, "(none)")
	} else {
		fmt.Fprintln(c.stdout, strings.Join(versions, "\n"))
	}
	fmt.Fprintln(c.stdout)
	c.help()
	return nil
}

func (c *Command) list() error {
	versions, err := c.manager.List()
	if err != nil {
		return err
	}

	for _, version := range versions {
		fmt.Fprintln(c.stdout, version)
	}

	return nil
}

func (c *Command) current() error {
	current, err := c.manager.Current()
	if err != nil {
		return err
	}

	fmt.Fprintf(c.stdout, "%s -> %s\n", current.Name, current.Path)
	return nil
}

func (c *Command) switchDefault(name string) error {
	current, err := c.manager.Switch(name)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "go-current -> %s\n", current.Path)
	return nil
}

func (c *Command) choose() error {
	versions, err := c.manager.List()
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		return fmt.Errorf("no installed Go SDKs found")
	}

	for i, name := range versions {
		fmt.Fprintf(c.stdout, "%d) %s\n", i+1, name)
	}
	fmt.Fprint(c.stdout, "Choose default Go version: ")

	line, err := bufio.NewReader(c.stdin).ReadString('\n')
	if err != nil && len(line) == 0 {
		return fmt.Errorf("read selection: %w", err)
	}
	choice, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || choice < 1 || choice > len(versions) {
		return fmt.Errorf("invalid selection")
	}

	return c.switchDefault(versions[choice-1])
}

func (c *Command) install(ctx context.Context, archive string) error {
	if strings.HasSuffix(archive, ".tar.gz") || strings.ContainsRune(archive, os.PathSeparator) {
		return c.installArchive(ctx, archive)
	}
	result, err := c.manager.InstallDownload(ctx, archive)
	if err != nil {
		return err
	}
	if result.Existed {
		fmt.Fprintf(c.stdout, "%s already exists\n", result.Version.Name)
	} else {
		fmt.Fprintf(c.stdout, "installed %s from %s\n", result.Version.Name, result.File)
	}
	fmt.Fprintf(c.stdout, "go-current -> %s\n", result.Version.Path)
	return nil
}

func (c *Command) installArchive(ctx context.Context, archive string) error {
	installed, existed, err := c.manager.InstallArchive(ctx, archive)
	if err != nil {
		return err
	}
	if existed {
		fmt.Fprintf(c.stdout, "%s already exists\n", installed.Name)
	} else {
		fmt.Fprintf(c.stdout, "installed %s\n", installed.Name)
	}
	fmt.Fprintf(c.stdout, "go-current -> %s\n", installed.Path)
	return nil
}

func (c *Command) selfInstall() error {
	result, err := c.manager.SelfInstall("")
	if err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "installed gosdkctl -> %s\n", result.BinaryPath)
	fmt.Fprintf(c.stdout, "go-sdk -> %s\n", result.AliasPath)
	return nil
}

func (c *Command) migrateLocal() error {
	migrated, err := c.manager.MigrateLocal()
	if err != nil {
		return err
	}
	if migrated.AlreadyMigrated {
		fmt.Fprintf(c.stdout, "%s already exists in %s\n", migrated.Version.Name, migrated.Version.Path)
	} else {
		fmt.Fprintf(c.stdout, "migrated ~/.local/go -> %s\n", migrated.Version.Path)
	}
	fmt.Fprintf(c.stdout, "go-current -> %s\n", migrated.Version.Path)
	return nil
}

func (c *Command) initShell(shell string) error {
	result, err := c.manager.InitShell(shell)
	if err != nil {
		return err
	}
	if result.Changed {
		fmt.Fprintf(c.stdout, "updated %s config: %s\n", result.Shell, result.Path)
	} else {
		fmt.Fprintf(c.stdout, "%s config already up to date: %s\n", result.Shell, result.Path)
	}
	return nil
}

func (c *Command) doctor() error {
	report, err := c.manager.Doctor()
	if err != nil {
		return err
	}
	fmt.Fprint(c.stdout, report)
	return nil
}

func (c *Command) env(target string) error {
	env, err := c.manager.Env(target)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.stdout, "export GOROOT=%q\n", env.GOROOT)
	fmt.Fprintf(c.stdout, "export GOPATH=%q\n", env.GOPATH)
	fmt.Fprintf(c.stdout, "export PATH=%q\n", env.PATH)
	return nil
}

func (c *Command) help() {
	fmt.Fprintln(c.stdout, "Usage:")
	fmt.Fprintln(c.stdout, "  gosdkctl status")
	fmt.Fprintln(c.stdout, "  gosdkctl list")
	fmt.Fprintln(c.stdout, "  gosdkctl current")
	fmt.Fprintln(c.stdout, "  gosdkctl switch <version>")
	fmt.Fprintln(c.stdout, "  gosdkctl install <archive|version|latest>")
	fmt.Fprintln(c.stdout, "  gosdkctl migrate-local")
	fmt.Fprintln(c.stdout, "  gosdkctl init [zsh|bash]")
	fmt.Fprintln(c.stdout, "  gosdkctl self install")
	fmt.Fprintln(c.stdout, "  gosdkctl choose")
	fmt.Fprintln(c.stdout, "  gosdkctl doctor")
	fmt.Fprintln(c.stdout, "  gosdkctl env [version|path|default]")
}
