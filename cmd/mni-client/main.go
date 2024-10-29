package main

import (
	"fmt"
	"github.com/mNi-Cloud/cli/internal/app"
	"os"
)

func main() {
	app := app.New()

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
