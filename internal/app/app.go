package app

import (
	"context"
	"fmt"
	"os"

	"github.com/rybalka1/gosdkctl/internal/cli"
	"github.com/rybalka1/gosdkctl/internal/config"
	"github.com/rybalka1/gosdkctl/internal/sdk"
)

func Run(args []string) int {
	cfg, err := config.Default()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	manager := sdk.NewManager(cfg)
	command := cli.New(manager, os.Stdin, os.Stdout, os.Stderr)

	if err := command.Run(context.Background(), args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}
