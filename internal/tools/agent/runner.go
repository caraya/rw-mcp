package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

const (
	defaultTimeout   = 45 * time.Second
	defaultMaxOutput = 32 * 1024
)

type Runner interface {
	Run(context.Context, string) (string, error)
}

type CommandRunner struct {
	command   string
	args      []string
	timeout   time.Duration
	maxOutput int
}

type boundedBuffer struct {
	buffer   bytes.Buffer
	maxBytes int
	overflow bool
}

func NewCommandRunner(command string, args []string, timeout time.Duration, maxOutput int) (*CommandRunner, error) {
	if strings.TrimSpace(command) == "" {
		return nil, errors.New("agent command must not be empty")
	}
	resolvedCommand, err := exec.LookPath(command)
	if err != nil {
		return nil, fmt.Errorf("resolve agent command: %w", err)
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if maxOutput <= 0 {
		maxOutput = defaultMaxOutput
	}
	return &CommandRunner{
		command:   resolvedCommand,
		args:      append([]string(nil), args...),
		timeout:   timeout,
		maxOutput: maxOutput,
	}, nil
}

func (r *CommandRunner) Run(ctx context.Context, prompt string) (string, error) {
	if strings.TrimSpace(prompt) == "" {
		return "", errors.New("agent prompt must not be empty")
	}
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	args := make([]string, 0, len(r.args)+1)
	args = append(args, r.args...)
	args = append(args, prompt)
	command := exec.CommandContext(ctx, r.command, args...)
	output := &boundedBuffer{maxBytes: r.maxOutput}
	command.Stdout = output
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("agent command stopped: %w", ctx.Err())
		}
		return "", errors.New("agent command failed")
	}
	if output.overflow {
		return "", fmt.Errorf("agent output exceeds %d bytes", r.maxOutput)
	}
	return output.buffer.String(), nil
}

func (b *boundedBuffer) Write(data []byte) (int, error) {
	remaining := b.maxBytes - b.buffer.Len()
	if remaining > 0 {
		writeLength := len(data)
		if writeLength > remaining {
			writeLength = remaining
		}
		_, _ = b.buffer.Write(data[:writeLength])
	}
	if len(data) > remaining {
		b.overflow = true
	}
	return len(data), nil
}
