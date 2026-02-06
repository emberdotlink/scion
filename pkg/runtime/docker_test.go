package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ptone/scion-agent/pkg/harness"
)

func TestDockerRuntime_Run_NoUserFlag(t *testing.T) {
	// We'll use a wrapper that just echo its arguments so we can inspect them
	// but we need it to look like a successful command execution.
	
	// Create a temporary script to act as a mock docker
	tmpDir := t.TempDir()
	mockDocker := filepath.Join(tmpDir, "mock-docker")
	
	script := `#!/bin/sh
echo "$@"
`
	if err := os.WriteFile(mockDocker, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write mock docker: %v", err)
	}

	runtime := &DockerRuntime{
		Command: mockDocker,
	}

	config := RunConfig{
		Harness:      &harness.GeminiCLI{},
		Name:         "test-agent",
		UnixUsername: "scion",
		Image:        "scion-agent:latest",
		Task:         "hello",
	}

	out, err := runtime.Run(context.Background(), config)
	if err != nil {
		t.Fatalf("runtime.Run failed: %v", err)
	}

	if strings.Contains(out, "--user scion:scion") {
		t.Errorf("expected '--user scion:scion' to be absent in output, got %q", out)
	}
	
	if !strings.Contains(out, "run --init -t") {
		t.Errorf("expected 'run --init -t' in output, got %q", out)
	}
}
