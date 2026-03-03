package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pinglo/internal/pinglo"
)

type waybarPayload struct {
	Text    string `json:"text"`
	Tooltip string `json:"tooltip"`
	Class   string `json:"class,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	socket := pinglo.DefaultSocketPath()
	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "start":
		fs := flag.NewFlagSet("start", flag.ExitOnError)
		cwd := fs.String("cwd", mustCwd(), "working directory")
		command := fs.String("cmd", "", "command string")
		socketFlag := fs.String("socket", socket, "unix socket path")
		_ = fs.Parse(args)
		if strings.TrimSpace(*command) == "" {
			die("start: --cmd is required")
		}
		sendOrDie(*socketFlag, pinglo.Request{Action: "start", Cwd: *cwd, Command: *command})
	case "done":
		fs := flag.NewFlagSet("done", flag.ExitOnError)
		cwd := fs.String("cwd", mustCwd(), "working directory")
		command := fs.String("cmd", "", "command string")
		exitCode := fs.Int("exit-code", 0, "process exit code")
		socketFlag := fs.String("socket", socket, "unix socket path")
		_ = fs.Parse(args)
		if strings.TrimSpace(*command) == "" {
			die("done: --cmd is required")
		}
		ec := *exitCode
		sendOrDie(*socketFlag, pinglo.Request{Action: "finish", Cwd: *cwd, Command: *command, ExitCode: &ec})
	case "clear":
		fs := flag.NewFlagSet("clear", flag.ExitOnError)
		socketFlag := fs.String("socket", socket, "unix socket path")
		_ = fs.Parse(args)
		sendOrDie(*socketFlag, pinglo.Request{Action: "clear"})
	case "list":
		fs := flag.NewFlagSet("list", flag.ExitOnError)
		socketFlag := fs.String("socket", socket, "unix socket path")
		_ = fs.Parse(args)
		resp := sendOrDie(*socketFlag, pinglo.Request{Action: "list"})
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(resp.Items)
	case "dot":
		if len(args) == 0 {
			die("dot command requires subcommand (set/remove)")
		}
		switch args[0] {
		case "set":
			fs := flag.NewFlagSet("dot set", flag.ExitOnError)
			id := fs.String("id", "", "dot ID (required)")
			color := fs.String("color", "", "dot color (hex) for the foreground")
			tooltip := fs.String("tooltip", "", "tooltip text")
			status := fs.String("status", "", "dot status: running|success|failed")
			socketFlag := fs.String("socket", socket, "unix socket path")
			_ = fs.Parse(args[1:])
			if strings.TrimSpace(*id) == "" {
				die("dot set: --id is required")
			}
			st, err := parseStatus(*status)
			if err != nil {
				die(err.Error())
			}
			sendOrDie(*socketFlag, pinglo.Request{
				Action: "dot",
				Dot:    &pinglo.DotRequest{ID: *id, Color: *color, Tooltip: *tooltip, Status: st},
			})
		case "remove":
			fs := flag.NewFlagSet("dot remove", flag.ExitOnError)
			id := fs.String("id", "", "dot ID (required)")
			socketFlag := fs.String("socket", socket, "unix socket path")
			_ = fs.Parse(args[1:])
			if strings.TrimSpace(*id) == "" {
				die("dot remove: --id is required")
			}
			sendOrDie(*socketFlag, pinglo.Request{Action: "dot-remove", Dot: &pinglo.DotRequest{ID: *id}})
		default:
			die("dot command must be set or remove")
		}
	case "render":
		fs := flag.NewFlagSet("render", flag.ExitOnError)
		mode := fs.String("format", "waybar", "render format: waybar|plain")
		socketFlag := fs.String("socket", socket, "unix socket path")
		_ = fs.Parse(args)
		resp := sendOrDie(*socketFlag, pinglo.Request{Action: "list"})
		switch *mode {
		case "plain":
			fmt.Println(renderPlain(resp.Items))
		case "waybar":
			payload := waybarPayload{
				Text:    renderDots(resp.Items),
				Tooltip: renderTooltip(resp.Items),
				Class:   renderClass(resp.Items),
			}
			if len(resp.Items) == 0 {
				payload.Class = "empty"
			}
			b, _ := json.Marshal(payload)
			fmt.Println(string(b))
		default:
			die("render: unsupported --format, use waybar or plain")
		}
	case "help", "-h", "--help":
		usage()
	default:
		die("unknown command: " + cmd)
	}
}

func sendOrDie(socket string, req pinglo.Request) pinglo.Response {
	resp, err := send(socket, req)
	if err != nil {
		die(err.Error())
	}
	if !resp.OK {
		die(resp.Error)
	}
	return resp
}

func send(socket string, req pinglo.Request) (pinglo.Response, error) {
	conn, err := net.DialTimeout("unix", socket, 800*time.Millisecond)
	if err != nil {
		return pinglo.Response{}, fmt.Errorf("cannot connect to pinglod (%s): %w", socket, err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(1500 * time.Millisecond)); err != nil {
		_ = conn.Close()
		return pinglo.Response{}, fmt.Errorf("cannot set request deadline: %w", err)
	}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return pinglo.Response{}, err
	}

	var resp pinglo.Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return pinglo.Response{}, err
	}
	return resp, nil
}

func renderClass(items []pinglo.Item) string {
	if len(items) == 0 {
		return "empty"
	}
	hasFailed := false
	for _, item := range items {
		switch item.Status {
		case pinglo.StatusRunning:
			return "running"
		case pinglo.StatusFailed:
			hasFailed = true
		}
	}
	if hasFailed {
		return "failed"
	}
	return "success"
}

func renderPlain(items []pinglo.Item) string {
	if len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, string(item.Status)+":"+item.Command)
	}
	return strings.Join(parts, " | ")
}

func renderDots(items []pinglo.Item) string {
	if len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, renderDot(item))
	}
	return strings.Join(parts, " ")
}

func renderTooltip(items []pinglo.Item) string {
	if len(items) == 0 {
		return "No running/finished commands"
	}
	lines := make([]string, 0, len(items))
	for i, item := range items {
		tooltip := item.Tooltip
		if tooltip == "" {
			parts := make([]string, 0, 2)
			if item.Command != "" {
				parts = append(parts, item.Command)
			}
			if item.Cwd != "" {
				parts = append(parts, item.Cwd)
			}
			tooltip = strings.Join(parts, "\n")
			if tooltip == "" {
				tooltip = "<no details>"
			}
		}
		lines = append(lines, fmt.Sprintf("%d. [%s] %s", i+1, item.Status, tooltip))
	}
	return strings.Join(lines, "\n")
}

func renderDot(item pinglo.Item) string {
	color := item.Color
	if color == "" {
		color = defaultColorForStatus(item.Status)
	}
	return fmt.Sprintf(`<span foreground="%s">●</span>`, color)
}

func defaultColorForStatus(status pinglo.Status) string {
	switch status {
	case pinglo.StatusSuccess:
		return "#98c379"
	case pinglo.StatusFailed:
		return "#e06c75"
	default:
		return "#e5c07b"
	}
}

func parseStatus(in string) (pinglo.Status, error) {
	switch strings.TrimSpace(strings.ToLower(in)) {
	case "", "running":
		return pinglo.StatusRunning, nil
	case "success", "ok":
		return pinglo.StatusSuccess, nil
	case "failed", "failure":
		return pinglo.StatusFailed, nil
	default:
		return "", fmt.Errorf("unknown status: %s", in)
	}
}

func dotForStatus(s pinglo.Status) string {
	switch s {
	case pinglo.StatusRunning:
		return `<span foreground="#e5c07b">●</span>`
	case pinglo.StatusSuccess:
		return `<span foreground="#98c379">●</span>`
	case pinglo.StatusFailed:
		return `<span foreground="#e06c75">●</span>`
	default:
		return `<span foreground="#abb2bf">●</span>`
	}
}

func mustCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	p, err := filepath.Abs(cwd)
	if err != nil {
		return cwd
	}
	return p
}

func die(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}

func usage() {
	fmt.Println(`pinglo - CLI for pinglod

Usage:
  pinglo start --cmd "make test" [--cwd DIR] [--socket PATH]
  pinglo done --cmd "make test" --exit-code 0 [--cwd DIR] [--socket PATH]
  pinglo clear [--socket PATH]
  pinglo list [--socket PATH]
  pinglo dot set --id ID [--color COLOR] [--tooltip TEXT] [--status running|success|failed] [--socket PATH]
  pinglo dot remove --id ID [--socket PATH]
  pinglo render [--format waybar|plain] [--socket PATH]

Examples:
  pinglo start --cmd "sleep 20"
  pinglo dot set --id custom --color "#ff0" --tooltip "custom work"
  pinglo dot remove --id custom
  pinglo done --cmd "sleep 20" --exit-code 0

Tip:
  use shell hooks to automate start/done calls.`)
}
