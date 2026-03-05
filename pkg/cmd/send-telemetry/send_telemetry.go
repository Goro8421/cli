package sendtelemetry

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cli/cli/v2/internal/telemetry"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/spf13/cobra"
)

const defaultCentralEndpointURL = "https://central.github.com/api/usage/github-cli"

func NewCmdSendTelemetry(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "send-telemetry",
		Short:  "Send telemetry event to Central",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			endpointURL := defaultCentralEndpointURL
			if v := os.Getenv("CENTRAL_ENDPOINT_URL"); v != "" {
				endpointURL = v
			}
			return sendTelemetry(endpointURL, args[0])
		},
	}

	cmdutil.DisableAuthCheck(cmd)

	return cmd
}

func sendTelemetry(endpointURL, payloadJSON string) error {
	// Validate that the payload is a well-formed Event.
	var event telemetry.Event
	if err := json.Unmarshal([]byte(payloadJSON), &event); err != nil {
		return nil //nolint:nilerr // Best effort telemetry — silently discard bad payloads.
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequest(http.MethodPost, endpointURL, strings.NewReader(payloadJSON))
	if err != nil {
		return nil //nolint:nilerr // Best effort telemetry.
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil //nolint:nilerr // Best effort telemetry.
	}
	defer resp.Body.Close()

	return nil
}
