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

type SendTelemetryOptions struct {
	CentralEndpointURL string
	PayloadJSON        string
}

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

			opts := &SendTelemetryOptions{
				CentralEndpointURL: endpointURL,
				PayloadJSON:        args[0],
			}
			return runSendTelemetry(opts)
		},
	}

	cmdutil.DisableAuthCheck(cmd)

	return cmd
}

func runSendTelemetry(opts *SendTelemetryOptions) error {
	var event telemetry.Event
	if err := json.Unmarshal([]byte(opts.PayloadJSON), &event); err != nil {
		return nil //nolint:nilerr // Best effort telemetry.
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequest(http.MethodPost, opts.CentralEndpointURL, strings.NewReader(opts.PayloadJSON))
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
