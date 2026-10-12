package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCommandRunnerPassesPromptAsOneArgument(t *testing.T) {
	command := filepath.Join(t.TempDir(), "agent")
	script := "#!/bin/sh\nprintf '%s' \"$1\"\n"
	if err := os.WriteFile(command, []byte(script), 0o700); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	runner, err := NewCommandRunner(command, nil, 5*time.Second, 1024)
	if err != nil {
		t.Fatalf("NewCommandRunner() error = %v", err)
	}
	prompt := "summarize $(touch /tmp/should-not-exist)"
	got, err := runner.Run(context.Background(), prompt)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got != prompt {
		t.Fatalf("Run() = %q, want %q", got, prompt)
	}
}

func TestCommandRunnerRejectsOversizedOutput(t *testing.T) {
	command := filepath.Join(t.TempDir(), "agent")
	if err := os.WriteFile(command, []byte("#!/bin/sh\nprintf '12345'\n"), 0o700); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	runner, err := NewCommandRunner(command, nil, time.Second, 4)
	if err != nil {
		t.Fatalf("NewCommandRunner() error = %v", err)
	}
	if _, err := runner.Run(context.Background(), "hello"); err == nil {
		t.Fatal("Run() error = nil, want output-limit error")
	}
}
