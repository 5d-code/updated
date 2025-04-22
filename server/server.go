package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
)

const (
	socketPath  = "/run/updated.sock"
	tempUser    = "updateduser"
	homeDir     = "/home/" + tempUser
	sudoersFile = "/etc/sudoers.d/" + tempUser
	hookDir     = "/etc/updated/hooks"
	hookStart   = "on_update_start"
	hookDone    = "on_update_done"
)

func ensureHooksExist() {
	hooks := []string{hookStart, hookDone}
	_ = os.MkdirAll(hookDir, 0755)
	for _, hook := range hooks {
		path := filepath.Join(hookDir, hook)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			defaultContent := []byte("#!/usr/bin/sh\n")
			_ = os.WriteFile(path, defaultContent, 0755)
			log.Printf("Created default hook: %s", path)
		}
	}
}

func cleanupSocket() {
	if err := os.RemoveAll(socketPath); err != nil {
		log.Printf("Failed to remove socket: %v", err)
	}
}

func runHook(hook string) {
	hookPath := filepath.Join(hookDir, hook)
	if stat, err := os.Stat(hookPath); err == nil && stat.Mode()&0111 != 0 {
		log.Printf("Running hook: %s", hook)
		if err := exec.Command(hookPath).Run(); err != nil {
			log.Printf("Hook %s failed: %v", hook, err)
		}
	}
}

func doUpdate() {
	createdUser := false
	addedSudo := false

	defer func() {
		runHook(hookDone)
		if addedSudo {
			_ = os.Remove(sudoersFile)
			log.Printf("Removed sudoers file: %s", sudoersFile)
		}
		if createdUser {
			exec.Command("userdel", tempUser).Run()
			log.Printf("Deleted user: %s", tempUser)
			os.RemoveAll(homeDir)
			log.Printf("Removed home directory: %s", homeDir)
		}
	}()

	log.Printf("Creating temporary user: %s", tempUser)
	if err := exec.Command("useradd", "-m", "-d", homeDir, tempUser).Run(); err != nil {
		log.Printf("Failed to create user: %v", err)
		return
	}
	createdUser = true

	log.Printf("Granting sudo access")
	sudoers := fmt.Sprintf("%s ALL=(ALL) NOPASSWD: ALL\n", tempUser)
	if err := os.WriteFile(sudoersFile, []byte(sudoers), 0440); err != nil {
		log.Printf("Failed to write sudoers file: %v", err)
		return
	}
	addedSudo = true

	runHook(hookStart)

	log.Printf("Running paru update")
	cmd := exec.Command("runuser", "-l", tempUser, "-c", "paru -Syu --noconfirm")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("Update failed: %v", err)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		log.Printf("Failed to read: %v", err)
		return
	}
	msg := string(buf[:n])
	log.Printf("Received command: %s", msg)
	if msg == "update" {
		doUpdate()
		_, _ = conn.Write([]byte("OK\n"))
	} else {
		_, _ = conn.Write([]byte("ERROR: unknown command\n"))
	}
}

func Run() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	cleanupSocket()
	ensureHooksExist()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		cleanupSocket()
		os.Exit(0)
	}()

	l, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer l.Close()
	log.Printf("Daemon listening on %s", socketPath)

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}
