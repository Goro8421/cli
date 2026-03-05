// Package telemetry provides best-effort usage telemetry for gh commands.
//
// Telemetry is sent by spawning a detached `gh send-telemetry` subprocess from
// a PersistentPostRun hook on the root cobra command. This has several known
// limitations:
//
//   - Telemetry is only sent on successful command completion. Commands that
//     are interrupted (e.g. Ctrl+C) or fail with an error do not trigger the
//     PersistentPostRun hook, so no event is recorded.
//   - There is no opt-out mechanism yet. This should be added before shipping
//     to public users (e.g. via a config setting or environment variable).
package telemetry

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/cli/cli/v2/internal/config"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

const deviceIDFileName = "device-id"

// deviceIDFunc returns a per-user device identifier stored in the state directory.
// It generates and persists a UUID on first call. Can be replaced in tests.
var deviceIDFunc = func() (string, error) {
	return getOrCreateDeviceID()
}

func getOrCreateDeviceID() (string, error) {
	idPath := filepath.Join(config.StateDir(), deviceIDFileName)

	data, err := os.ReadFile(idPath)
	if err == nil {
		return string(data), nil
	}

	id := uuid.New().String()
	if err := os.MkdirAll(config.StateDir(), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(idPath, []byte(id), 0o600); err != nil {
		return "", err
	}
	return id, nil
}

// Event represents a Central usage event payload.
// This struct is marshaled to JSON by the caller and unmarshaled by the
// send-telemetry command, providing type safety across the process boundary.
type Event struct {
	EventType  string     `json:"eventType"`
	Dimensions Dimensions `json:"dimensions"`
}

// Dimensions contains the metadata sent alongside a usage event to Central.
type Dimensions struct {
	Command      string `json:"command"`
	DeviceID     string `json:"device_id"`
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
	Version      string `json:"version"`
}

// BuildEventPayload constructs the event payload for tracking a command invocation.
// Returns nil if cmd is nil or the device ID cannot be determined.
func BuildEventPayload(cmd *cobra.Command, version string) *Event {
	if cmd == nil {
		return nil
	}

	deviceID, err := deviceIDFunc()
	if err != nil {
		return nil
	}

	return &Event{
		EventType: "usage",
		Dimensions: Dimensions{
			Command:      cmd.CommandPath(),
			DeviceID:     deviceID,
			Platform:     runtime.GOOS,
			Architecture: runtime.GOARCH,
			Version:      version,
		},
	}
}

// SpawnSendTelemetry spawns a subprocess to send telemetry.
// All errors are silently ignored since telemetry is best-effort.
func SpawnSendTelemetry(executable, payloadJSON string) {
	cmd := exec.Command(executable, "send-telemetry", payloadJSON)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return
	}
	_ = cmd.Process.Release() //nolint:errcheck // Best effort telemetry.
}

const telemetryAnnotation = "telemetry"

// EnableTelemetry opts a command into telemetry collection.
//
// During the initial rollout, telemetry is opt-in per command. In the future,
// the default should be swapped so that telemetry is enabled for all commands
// unless explicitly disabled.
func EnableTelemetry(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[telemetryAnnotation] = "true"
}

// IsTelemetryEnabled checks whether telemetry is enabled for the given command.
func IsTelemetryEnabled(cmd *cobra.Command) bool {
	return cmd.Annotations != nil && cmd.Annotations[telemetryAnnotation] == "true"
}
