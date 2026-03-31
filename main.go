package main

import "github.com/grahambrooks/devc/cmd"

// version is set at build time via ldflags -X main.version=YYYY.MM.DD
var version = "dev"

func main() {
	cmd.Execute(version)
}
