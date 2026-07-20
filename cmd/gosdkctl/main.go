package main

import (
	"os"

	"github.com/rybalka1/gosdkctl/internal/app"
)

func main() {
	os.Exit(app.Run(os.Args[1:]))
}
