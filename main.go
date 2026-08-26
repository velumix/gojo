package main

import (
	"os"

	"github.com/velumix/gojo/internal/app"
)

func main() {
	app.Run(os.Args[1:])
}
