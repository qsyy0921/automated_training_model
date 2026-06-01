package main

import (
	"fmt"
	"os"

	"github.com/qsyy0921/automated_training_model/internal/cli/labelctl"
)

func main() {
	if err := labelctl.Run(nil); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
