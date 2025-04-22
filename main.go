package main

import (
	"fmt"
	"os"

	"github.com/5d-code/updated/client"
	"github.com/5d-code/updated/server"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: updated <server|message>")
		os.Exit(1)
	}

	if os.Args[1] == "server" {
		server.Run()
	} else {
		client.Run(os.Args[1:])
	}
}
