package main

import (
	"github.com/kopiro/dfman/internal/dfman"
	"os"
)

var version = "dev"

func main() { os.Exit(dfman.Execute(os.Args[1:], version, os.Stdin, os.Stdout, os.Stderr)) }
