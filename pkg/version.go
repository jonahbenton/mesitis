package pkg

import (
	"fmt"
	"os"
)

var VERSION = "0.1.0"

// PrintAndExit will print the version and exit.
func PrintAndExit() {
	fmt.Println(VERSION)
	os.Exit(0)
}
