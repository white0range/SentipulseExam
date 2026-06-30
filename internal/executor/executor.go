package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"sentipulseexam/internal/model"
	"sentipulseexam/pkg/protocol"
)

func Execute(parent context.Context, descriptor model.PluginDescriptor, request protocol.ExecuteRequest) model.PluginExecution {
	started := time.Now()

	payload, err := json.Marshal(request)
	if err != nil {
		return failedExecution(descriptor, model.ExecutionStatusFailed, time.Since(started), fmt.Sprintf("marshal request: %v", err))
	}

	timeout := time.Duration(descriptor.Manifest.TimeoutMS) * time.Millisecond
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	command := exec.CommandContext(ctx, descriptor.Manifest.Command[0], descriptor.Manifest.Command[1:]...)
	command.Dir = descriptor.WorkDir
	command.Stdin = bytes.NewReader(payload)
	command.Env = pluginEnvironment(descriptor)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err = command.Run()
	duration := time.Since(started)

	if ctx.Err() == context.DeadlineExceeded {
		return failedExecution(descriptor, model.ExecutionStatusTimeout, duration, fmt.Sprintf("plugin timed out after %dms", descriptor.Manifest.TimeoutMS))
	}

	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return failedExecution(descriptor, model.ExecutionStatusFailed, duration, message)
	}

	var response protocol.ExecuteResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		message := fmt.Sprintf("invalid plugin response: %v", err)
		if trimmed := strings.TrimSpace(stdout.String()); trimmed != "" {
			message = fmt.Sprintf("%s; stdout=%q", message, trimmed)
		}
		return failedExecution(descriptor, model.ExecutionStatusFailed, duration, message)
	}

	if !response.Success {
		message := response.Error
		if message == "" {
			message = "plugin returned failure"
		}
		return model.PluginExecution{
			Name:       descriptor.Manifest.Name,
			Version:    descriptor.Manifest.Version,
			Status:     model.ExecutionStatusFailed,
			DurationMS: duration.Milliseconds(),
			Result:     response.Result,
			Error:      message,
		}
	}

	return model.PluginExecution{
		Name:       descriptor.Manifest.Name,
		Version:    descriptor.Manifest.Version,
		Status:     model.ExecutionStatusSuccess,
		DurationMS: duration.Milliseconds(),
		Result:     response.Result,
	}
}

func failedExecution(descriptor model.PluginDescriptor, status model.ExecutionStatus, duration time.Duration, message string) model.PluginExecution {
	return model.PluginExecution{
		Name:       descriptor.Manifest.Name,
		Version:    descriptor.Manifest.Version,
		Status:     status,
		DurationMS: duration.Milliseconds(),
		Error:      message,
	}
}

func pluginEnvironment(descriptor model.PluginDescriptor) []string {
	env := os.Environ()

	if len(descriptor.Manifest.Command) > 1 && descriptor.Manifest.Command[0] == "go" && descriptor.Manifest.Command[1] == "run" {
		cacheRoot := filepath.Join(os.TempDir(), "sentipulseexam-go-cache", sanitizeEnvName(descriptor.Manifest.Name))
		tmpRoot := filepath.Join(os.TempDir(), "sentipulseexam-go-tmp", sanitizeEnvName(descriptor.Manifest.Name))
		_ = os.MkdirAll(cacheRoot, 0o755)
		_ = os.MkdirAll(tmpRoot, 0o755)

		env = append(env,
			fmt.Sprintf("GOCACHE=%s", cacheRoot),
			fmt.Sprintf("GOTMPDIR=%s", tmpRoot),
		)
	}

	return env
}

func sanitizeEnvName(name string) string {
	replacer := strings.NewReplacer(
		"\\", "-",
		"/", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
		" ", "-",
	)
	return replacer.Replace(name)
}
