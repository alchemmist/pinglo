package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"pinglo/internal/pinglo"
)

const sigRTMin = 34

func main() {
	var socket string
	var signalOffset int
	flag.StringVar(&socket, "socket", pinglo.DefaultSocketPath(), "unix socket path")
	flag.IntVar(&signalOffset, "signal-offset", 4, "signal offset from SIGRTMIN to refresh Waybar module")
	flag.Parse()

	if err := os.MkdirAll(filepath.Dir(socket), 0o755); err != nil {
		log.Fatalf("failed to create socket dir: %v", err)
	}
	_ = os.Remove(socket)

	listener, err := net.Listen("unix", socket)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", socket, err)
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(socket)
	}()
	_ = os.Chmod(socket, 0o600)

	mgr := pinglo.NewManager(makeWaybarNotifier(signalOffset))

	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigC
		_ = listener.Close()
	}()

	log.Printf("pinglod listening on %s", socket)
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			log.Printf("accept error: %v", err)
			continue
		}
		go handleConn(conn, mgr)
	}
}

func handleConn(conn net.Conn, mgr *pinglo.Manager) {
	defer conn.Close()

	var req pinglo.Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		if err != io.EOF {
			_ = json.NewEncoder(conn).Encode(pinglo.Response{OK: false, Error: fmt.Sprintf("decode error: %v", err)})
		}
		return
	}

	resp := dispatch(req, mgr)
	_ = json.NewEncoder(conn).Encode(resp)
}

func dispatch(req pinglo.Request, mgr *pinglo.Manager) pinglo.Response {
	req.Action = strings.TrimSpace(req.Action)
	req.Cwd = strings.TrimSpace(req.Cwd)
	req.Command = strings.TrimSpace(req.Command)

	switch req.Action {
	case "start":
		if req.Cwd == "" || req.Command == "" {
			return pinglo.Response{OK: false, Error: "start requires cwd and command"}
		}
		mgr.Start(req.Cwd, req.Command)
		return pinglo.Response{OK: true, Items: mgr.List()}
	case "finish":
		if req.Cwd == "" || req.Command == "" || req.ExitCode == nil {
			return pinglo.Response{OK: false, Error: "finish requires cwd, command, exit_code"}
		}
		mgr.Finish(req.Cwd, req.Command, *req.ExitCode)
		return pinglo.Response{OK: true, Items: mgr.List()}
	case "clear":
		mgr.Clear()
		return pinglo.Response{OK: true, Items: nil}
	case "list":
		return pinglo.Response{OK: true, Items: mgr.List()}
	default:
		return pinglo.Response{OK: false, Error: "unknown action"}
	}
}

func makeWaybarNotifier(offset int) func() {
	if offset < 0 || offset > 31 {
		return nil
	}
	sig := syscall.Signal(sigRTMin + offset)
	return func() {
		notifyWaybar(sig)
	}
}

func notifyWaybar(sig syscall.Signal) {
	out, err := exec.Command("pgrep", "waybar").Output()
	if err != nil {
		return
	}
	for _, line := range strings.Fields(strings.TrimSpace(string(out))) {
		pid, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		_ = syscall.Kill(pid, sig)
	}
}
