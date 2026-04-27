package main

import (
	// Make sure that package gen is initialized for sub commands
	_ "github.com/ancalabrese/ws1cli/internal/cli/gen"

	"github.com/ancalabrese/ws1cli/internal/cli"
)

func main() { cli.Execute() }
