package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/liliang-cn/tagit/internal/tagitpath"
)

const (
	daemonPIDFile  = "tagitd.pid"
	daemonLogFile  = "tagitd.log"
	daemonStopWait = 10 * time.Second
)

func daemonPIDPath() string {
	return filepath.Join(tagitpath.HomeDir(), daemonPIDFile)
}

func daemonLogPath() string {
	return filepath.Join(tagitpath.HomeDir(), daemonLogFile)
}

// daemonStatus checks whether tagitd is running by reading the PID file and
// signalling the process with signal 0. Returns (running, pid).
func daemonStatus() (bool, int) {
	data, err := os.ReadFile(daemonPIDPath())
	if err != nil {
		return false, 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return false, 0
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, 0
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false, 0
	}
	return true, pid
}

// findTagItdBinary locates the tagitd binary. It first checks the directory
// containing the current executable, then falls back to PATH.
func findTagItdBinary() (string, error) {
	self, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(self), "tagitd")
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	path, err := exec.LookPath("tagitd")
	if err != nil {
		return "", fmt.Errorf("tagitd binary not found alongside tagit or in PATH")
	}
	return path, nil
}

// parseStartArgs extracts recognized flags from args and returns tagitd argv.
func parseStartArgs(args []string) ([]string, error) {
	var tagitdArgs []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--acp-port":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("--acp-port requires a value")
			}
			tagitdArgs = append(tagitdArgs, "--acp-port", args[i])
		default:
			return nil, fmt.Errorf("unknown flag for start: %s", args[i])
		}
	}
	return tagitdArgs, nil
}

// runStart launches tagitd as a detached background process.
func runStart(args []string) error {
	if running, pid := daemonStatus(); running {
		fmt.Printf("tagitd is already running (pid=%d)\n", pid)
		return nil
	}

	tagitdPath, err := findTagItdBinary()
	if err != nil {
		return err
	}

	tagitdArgs, err := parseStartArgs(args)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(tagitpath.HomeDir(), 0o755); err != nil {
		return fmt.Errorf("create tagit home dir: %w", err)
	}

	logFile, err := os.OpenFile(daemonLogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file %s: %w", daemonLogPath(), err)
	}
	defer logFile.Close()

	cmd := exec.Command(tagitdPath, tagitdArgs...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start tagitd: %w", err)
	}

	pidData := strconv.Itoa(cmd.Process.Pid) + "\n"
	if err := os.WriteFile(daemonPIDPath(), []byte(pidData), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write pid file: %v\n", err)
	}

	fmt.Printf("tagitd started (pid=%d, log=%s)\n", cmd.Process.Pid, daemonLogPath())
	return nil
}

// runStop sends SIGTERM to the running tagitd and waits up to 10 seconds before
// sending SIGKILL. The PID file is removed on success.
func runStop() error {
	running, pid := daemonStatus()
	if !running {
		fmt.Println("tagitd is not running")
		return nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM to pid %d: %w", pid, err)
	}

	deadline := time.Now().Add(daemonStopWait)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			_ = os.Remove(daemonPIDPath())
			fmt.Printf("tagitd stopped (pid=%d)\n", pid)
			return nil
		}
	}

	_ = proc.Signal(syscall.SIGKILL)
	_ = os.Remove(daemonPIDPath())
	fmt.Printf("tagitd killed after %s timeout (pid=%d)\n", daemonStopWait, pid)
	return nil
}
