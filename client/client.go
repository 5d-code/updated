package client

import (
	"fmt"
	"net"
	"os"
	"strings"
)

const socket = "/run/updated.sock"

func Run(args []string) {
	msg := strings.Join(args, " ")

	conn, err := net.Dial("unix", socket)
	if err != nil {
		fmt.Printf("Failed to connect to daemon: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	_, err = conn.Write([]byte(msg))
	if err != nil {
		fmt.Printf("Failed to send message: %v\n", err)
		os.Exit(1)
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Printf("Failed to read response: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(string(buf[:n]))
}
