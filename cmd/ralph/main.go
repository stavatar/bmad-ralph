package main

import (
	_ "github.com/fatih/color"
	_ "github.com/spf13/cobra"
	_ "gopkg.in/yaml.v3"
)

// version is set by goreleaser ldflags at build time.
var version = "dev"

func main() {
}
