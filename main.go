package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var version = "dev"

const (
	brewingOutput = "☕ Brewing"
	restingOutput = "☕ Resting"
)

type command int

const (
	cmdToggle command = iota
	cmdOn
	cmdOff
	cmdStatus
	cmdHelp
	cmdVersion
)

type state struct {
	PID     int    `json:"pid"`
	Command string `json:"command"`
	Started string `json:"started"`
}

type processStarter interface {
	StartCaffeinate() (state, error)
	IsRunning(state) bool
	Terminate(state) error
}

type manager struct {
	statePath string
	processes processStarter
}

type realProcesses struct{}

func main() {
	statePath, err := defaultStatePath()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	m := manager{statePath: statePath, processes: realProcesses{}}
	stopSignalCleanup := installSignalCleanup(os.Args[1:], m)
	defer stopSignalCleanup()

	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, m))
}

func installSignalCleanup(args []string, m manager) func() {
	cmd, err := parseCommand(args)
	if err != nil {
		return func() {}
	}
	if cmd != cmdToggle && cmd != cmdOn && cmd != cmdOff {
		return func() {}
	}

	signals := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case <-signals:
			_, _ = m.Off()
			fmt.Fprintln(os.Stderr, "interrupted")
			os.Exit(1)
		case <-done:
			return
		}
	}()

	return func() {
		signal.Stop(signals)
		close(done)
	}
}

func run(args []string, stdout, stderr io.Writer, m manager) int {
	cmd, err := parseCommand(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	switch cmd {
	case cmdHelp:
		fmt.Fprint(stdout, helpText())
		return 0
	case cmdVersion:
		fmt.Fprintf(stdout, "coffee %s\n", version)
		return 0
	case cmdStatus:
		active, err := m.Status()
		return printResult(stdout, stderr, active, err)
	case cmdOn:
		active, err := m.On()
		return printResult(stdout, stderr, active, err)
	case cmdOff:
		active, err := m.Off()
		return printResult(stdout, stderr, active, err)
	case cmdToggle:
		active, err := m.Toggle()
		return printResult(stdout, stderr, active, err)
	default:
		fmt.Fprintln(stderr, "internal error: unknown command")
		return 1
	}
}

func printResult(stdout, stderr io.Writer, active bool, err error) int {
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if active {
		fmt.Fprintln(stdout, brewingOutput)
	} else {
		fmt.Fprintln(stdout, restingOutput)
	}
	return 0
}

func parseCommand(args []string) (command, error) {
	if len(args) == 0 {
		return cmdToggle, nil
	}
	if len(args) != 1 {
		return 0, fmt.Errorf("invalid arguments: %s", strings.Join(args, " "))
	}

	switch args[0] {
	case "on":
		return cmdOn, nil
	case "off":
		return cmdOff, nil
	case "status":
		return cmdStatus, nil
	case "--help", "-h":
		return cmdHelp, nil
	case "--version":
		return cmdVersion, nil
	default:
		return 0, fmt.Errorf("invalid command: %s", args[0])
	}
}

func helpText() string {
	return `Usage:
  coffee
  coffee on
  coffee off
  coffee status
  coffee --help
  coffee --version

Toggle temporary macOS idle sleep prevention using caffeinate.

Output:
  ☕ Brewing  Coffee is active.
  ☕ Resting  Coffee is inactive.
`
}

func defaultStatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, "Library", "Application Support", "coffee", "coffee.pid"), nil
}

func (m manager) Toggle() (bool, error) {
	active, err := m.Status()
	if err != nil {
		return false, err
	}
	if active {
		return m.Off()
	}
	return m.On()
}

func (m manager) On() (bool, error) {
	current, active, err := m.currentState()
	if err != nil {
		return false, err
	}
	if active {
		return true, nil
	}
	if current.PID != 0 {
		if err := m.removeState(); err != nil {
			return false, err
		}
	}

	next, err := m.processes.StartCaffeinate()
	if err != nil {
		return false, fmt.Errorf("start caffeinate: %w", err)
	}
	if err := m.writeState(next); err != nil {
		_ = m.processes.Terminate(next)
		return false, err
	}
	return true, nil
}

func (m manager) Off() (bool, error) {
	current, active, err := m.currentState()
	if err != nil {
		return false, err
	}
	if !active {
		if current.PID != 0 {
			if err := m.removeState(); err != nil {
				return false, err
			}
		}
		return false, nil
	}

	if err := m.processes.Terminate(current); err != nil {
		return false, fmt.Errorf("stop caffeinate: %w", err)
	}
	if err := m.removeState(); err != nil {
		return false, err
	}
	return false, nil
}

func (m manager) Status() (bool, error) {
	current, active, err := m.currentState()
	if err != nil {
		return false, err
	}
	if !active && current.PID != 0 {
		if err := m.removeState(); err != nil {
			return false, err
		}
	}
	return active, nil
}

func (m manager) currentState() (state, bool, error) {
	current, err := m.readState()
	if err != nil {
		return state{}, false, err
	}
	if current.PID == 0 {
		return state{}, false, nil
	}
	return current, m.processes.IsRunning(current), nil
}

func (m manager) readState() (state, error) {
	content, err := os.ReadFile(m.statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state{}, nil
		}
		return state{}, fmt.Errorf("read state: %w", err)
	}

	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		return state{}, nil
	}

	if strings.HasPrefix(trimmed, "{") {
		var current state
		if err := json.Unmarshal([]byte(trimmed), &current); err != nil {
			return state{}, fmt.Errorf("parse state: %w", err)
		}
		if current.PID <= 0 || current.Command == "" || current.Started == "" {
			return state{}, fmt.Errorf("parse state: incomplete state file")
		}
		return current, nil
	}

	pid, err := strconv.Atoi(trimmed)
	if err != nil || pid <= 0 {
		return state{}, fmt.Errorf("parse state: invalid pid")
	}
	return state{PID: pid, Command: "caffeinate"}, nil
}

func (m manager) writeState(next state) error {
	if err := os.MkdirAll(filepath.Dir(m.statePath), 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	content, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	content = append(content, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(m.statePath), ".coffee.pid.")
	if err != nil {
		return fmt.Errorf("create temporary state file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary state file: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temporary state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary state file: %w", err)
	}
	if err := os.Rename(tmpPath, m.statePath); err != nil {
		return fmt.Errorf("replace state file: %w", err)
	}
	return nil
}

func (m manager) removeState() error {
	err := os.Remove(m.statePath)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("remove state: %w", err)
}

func (realProcesses) StartCaffeinate() (state, error) {
	cmd := exec.Command("caffeinate", "-i")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return state{}, err
	}

	pid := cmd.Process.Pid
	started, command, err := processIdentity(pid)
	if err != nil {
		_ = cmd.Process.Kill()
		return state{}, err
	}
	if command != "caffeinate" {
		_ = cmd.Process.Kill()
		return state{}, fmt.Errorf("started unexpected command %q", command)
	}
	if err := cmd.Process.Release(); err != nil {
		_ = cmd.Process.Kill()
		return state{}, err
	}

	return state{PID: pid, Command: command, Started: started}, nil
}

func (realProcesses) IsRunning(expected state) bool {
	started, command, err := processIdentity(expected.PID)
	if err != nil {
		return false
	}
	if command != expected.Command {
		return false
	}
	if expected.Started != "" && started != expected.Started {
		return false
	}
	return true
}

func (p realProcesses) Terminate(target state) error {
	if !p.IsRunning(target) {
		return nil
	}

	proc, err := os.FindProcess(target.PID)
	if err != nil {
		return err
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !p.IsRunning(target) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err := proc.Signal(syscall.SIGKILL); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	for range 20 {
		if !p.IsRunning(target) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("process %d did not exit", target.PID)
}

func processIdentity(pid int) (started string, command string, err error) {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=", "-o", "lstart=").Output()
	if err != nil {
		return "", "", err
	}
	fields := strings.Fields(string(out))
	if len(fields) < 6 {
		return "", "", fmt.Errorf("unexpected ps output for pid %d", pid)
	}
	command = filepath.Base(fields[0])
	started = strings.Join(fields[1:], " ")
	return started, command, nil
}
