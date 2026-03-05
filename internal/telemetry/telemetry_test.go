package telemetry

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cli/cli/v2/internal/config"
	"github.com/spf13/cobra"
)

func stubDeviceID(id string) func() {
	orig := deviceIDFunc
	deviceIDFunc = func() (string, error) { return id, nil }
	return func() { deviceIDFunc = orig }
}

func stubDeviceIDError() func() {
	orig := deviceIDFunc
	deviceIDFunc = func() (string, error) { return "", errors.New("no machine id") }
	return func() { deviceIDFunc = orig }
}

func TestBuildEventPayloadNilCommand(t *testing.T) {
	if event := BuildEventPayload(nil, "2.0.0"); event != nil {
		t.Errorf("expected nil for nil command, got %+v", event)
	}
}

func TestBuildEventPayloadPopulatesDimensions(t *testing.T) {
	cleanup := stubDeviceID("test-device-id")
	defer cleanup()

	root := &cobra.Command{Use: "gh"}
	pr := &cobra.Command{Use: "pr"}
	create := &cobra.Command{Use: "create"}
	root.AddCommand(pr)
	pr.AddCommand(create)

	event := BuildEventPayload(create, "2.45.0")
	if event == nil {
		t.Fatal("expected non-nil event")
	}

	if event.EventType != "usage" {
		t.Errorf("EventType = %q, want %q", event.EventType, "usage")
	}
	if event.Dimensions.Command != "gh pr create" {
		t.Errorf("Command = %q, want %q", event.Dimensions.Command, "gh pr create")
	}
	if event.Dimensions.DeviceID != "test-device-id" {
		t.Errorf("DeviceID = %q, want %q", event.Dimensions.DeviceID, "test-device-id")
	}
	if event.Dimensions.Platform != runtime.GOOS {
		t.Errorf("Platform = %q, want %q", event.Dimensions.Platform, runtime.GOOS)
	}
	if event.Dimensions.Architecture != runtime.GOARCH {
		t.Errorf("Architecture = %q, want %q", event.Dimensions.Architecture, runtime.GOARCH)
	}
	if event.Dimensions.Version != "2.45.0" {
		t.Errorf("Version = %q, want %q", event.Dimensions.Version, "2.45.0")
	}
}

func TestBuildEventPayloadReturnsNilWhenDeviceIDFails(t *testing.T) {
	cleanup := stubDeviceIDError()
	defer cleanup()

	cmd := &cobra.Command{Use: "gh"}
	event := BuildEventPayload(cmd, "2.45.0")
	if event != nil {
		t.Errorf("expected nil when device ID fails, got %+v", event)
	}
}

func TestBuildEventPayloadCommandPath(t *testing.T) {
	cleanup := stubDeviceID("test-device-id")
	defer cleanup()

	tests := []struct {
		name  string
		setup func() *cobra.Command
		want  string
	}{
		{
			name: "subcommand",
			setup: func() *cobra.Command {
				root := &cobra.Command{Use: "gh"}
				issue := &cobra.Command{Use: "issue"}
				list := &cobra.Command{Use: "list"}
				root.AddCommand(issue)
				issue.AddCommand(list)
				return list
			},
			want: "gh issue list",
		},
		{
			name: "top-level command",
			setup: func() *cobra.Command {
				root := &cobra.Command{Use: "gh"}
				status := &cobra.Command{Use: "status"}
				root.AddCommand(status)
				return status
			},
			want: "gh status",
		},
		{
			name: "root command itself",
			setup: func() *cobra.Command {
				return &cobra.Command{Use: "gh"}
			},
			want: "gh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.setup()
			event := BuildEventPayload(cmd, "1.0.0")
			if event == nil {
				t.Fatal("expected non-nil event")
			}
			if event.Dimensions.Command != tt.want {
				t.Errorf("Command = %q, want %q", event.Dimensions.Command, tt.want)
			}
		})
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	event := Event{
		EventType: "usage",
		Dimensions: Dimensions{
			Command:      "repo clone",
			DeviceID:     "abc123hashed",
			Platform:     "linux",
			Architecture: "amd64",
			Version:      "2.44.0",
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if decoded != event {
		t.Errorf("round-trip mismatch:\n  got:  %+v\n  want: %+v", decoded, event)
	}
}

func TestGetOrCreateDeviceID(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)
	// Also clear GH_CONFIG_DIR so it doesn't interfere with state dir resolution.
	t.Setenv("GH_CONFIG_DIR", "")

	t.Run("creates new ID on first call", func(t *testing.T) {
		id, err := getOrCreateDeviceID()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id == "" {
			t.Fatal("expected non-empty ID")
		}

		// Verify the file was persisted.
		data, err := os.ReadFile(filepath.Join(config.StateDir(), deviceIDFileName))
		if err != nil {
			t.Fatalf("failed to read device-id file: %v", err)
		}
		if string(data) != id {
			t.Errorf("file content = %q, want %q", string(data), id)
		}
	})

	t.Run("returns same ID on subsequent calls", func(t *testing.T) {
		id1, err := getOrCreateDeviceID()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		id2, err := getOrCreateDeviceID()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id1 != id2 {
			t.Errorf("IDs differ: %q vs %q", id1, id2)
		}
	})
}
