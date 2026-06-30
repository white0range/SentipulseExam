package main

import (
	"os"

	"sentipulseexam/internal/app"
)

func main() {
	os.Exit(app.Run(os.Args[1:]))
}
