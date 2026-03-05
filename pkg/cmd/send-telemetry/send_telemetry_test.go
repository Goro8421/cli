package sendtelemetry

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cli/cli/v2/internal/telemetry"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/spf13/cobra"
)

func TestSendTelemetryPostsToFakeCentral(t *testing.T) {
	var receivedBody []byte
	var receivedContentType string
	var receivedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedContentType = r.Header.Get("Content-Type")
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":"Cool usage bro."}`))
	}))
	defer server.Close()

	t.Setenv("CENTRAL_ENDPOINT_URL", server.URL+"/api/usage/github-cli")

	event := &telemetry.Event{
		EventType: "usage",
		Dimensions: telemetry.Dimensions{
			Command:      "pr create",
			DeviceID:     "abc123hashed",
			Platform:     "darwin",
			Architecture: "arm64",
			Version:      "2.45.0",
		},
	}

	payloadJSON, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	cmd := newTestCommand(string(payloadJSON))
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the request was made correctly.
	if receivedPath != "/api/usage/github-cli" {
		t.Errorf("expected path /api/usage/github-cli, got %s", receivedPath)
	}
	if receivedContentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", receivedContentType)
	}

	// Verify the payload matches what was sent.
	var received telemetry.Event
	if err := json.Unmarshal(receivedBody, &received); err != nil {
		t.Fatalf("failed to unmarshal received body: %v", err)
	}
	if received.EventType != "usage" {
		t.Errorf("expected eventType 'usage', got %q", received.EventType)
	}
	if received.Dimensions.Command != "pr create" {
		t.Errorf("expected command 'pr create', got %q", received.Dimensions.Command)
	}
	if received.Dimensions.Platform != "darwin" {
		t.Errorf("expected platform 'darwin', got %q", received.Dimensions.Platform)
	}
	if received.Dimensions.Architecture != "arm64" {
		t.Errorf("expected architecture 'arm64', got %q", received.Dimensions.Architecture)
	}
	if received.Dimensions.Version != "2.45.0" {
		t.Errorf("expected version '2.45.0', got %q", received.Dimensions.Version)
	}
}

func TestSendTelemetryInvalidJSON(t *testing.T) {
	cmd := newTestCommand("not valid json")
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error for invalid JSON (best-effort), got: %v", err)
	}
}

func TestSendTelemetryServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	t.Setenv("CENTRAL_ENDPOINT_URL", server.URL+"/api/usage/github-cli")

	event := &telemetry.Event{
		EventType: "usage",
		Dimensions: telemetry.Dimensions{
			Command: "pr list",
		},
	}

	payloadJSON, _ := json.Marshal(event)
	cmd := newTestCommand(string(payloadJSON))
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error for server error (best-effort), got: %v", err)
	}
}

func TestEventRoundTrip(t *testing.T) {
	original := telemetry.Event{
		EventType: "usage",
		Dimensions: telemetry.Dimensions{
			Command:      "issue create",
			Platform:     "linux",
			Architecture: "amd64",
			Version:      "2.44.0",
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded telemetry.Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded != original {
		t.Errorf("round-trip mismatch:\n  got:  %+v\n  want: %+v", decoded, original)
	}
}

// newTestCommand creates a send-telemetry cobra command wired up for testing.
func newTestCommand(payloadJSON string) *cobra.Command {
	ios, _, _, _ := iostreams.Test()
	f := &cmdutil.Factory{
		IOStreams: ios,
	}
	cmd := NewCmdSendTelemetry(f)
	cmd.SetArgs([]string{payloadJSON})
	return cmd
}
