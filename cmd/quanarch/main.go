// Command quanarch is the Quanarch command-line interface.
package main

import (
	"os"

	"github.com/quanarch/quanarch/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
