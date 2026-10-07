package util

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Result struct {
	Command string `json:"command"`
	Stdout  string `json:"stdout"`
	Stderr  string `json:"stderr"`
	Code    int    `json:"code"`
}

func Run(timeout time.Duration, name string, args ...string) Result {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = 127
		}
	}
	if ctx.Err() == context.DeadlineExceeded {
		code = 124
		if errb.Len() > 0 {
			errb.WriteByte('\n')
		}
		errb.WriteString("command timed out")
	}
	return Result{Command: strings.Join(append([]string{name}, args...), " "), Stdout: strings.TrimSpace(out.String()), Stderr: strings.TrimSpace(errb.String()), Code: code}
}

func RunStreaming(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Exists(name string) bool { _, err := exec.LookPath(name); return err == nil }

func RequireRoot() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("this operation changes the system and requires root privileges; run it again with sudo")
	}
	return nil
}

func RunStreamingDir(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
