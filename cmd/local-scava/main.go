// Command local-scava is a local-first daemon that tracks the career-growth
// routine (Monthly Skill Sprint + Three-Tier Content Cadence) and serves a
// monochrome, SRE-style dashboard at http://localhost:5500.
package main

import (
	"fmt"
	"os"
)

// version is overridable at build time via -ldflags "-X main.version=...".
var version = "0.0.0-dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "local-scava:", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Printf("local-scava %s (scaffold)\n", version)
	return nil
}
