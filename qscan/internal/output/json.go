package output

import (
	"encoding/json"
	"fmt"

	"github.com/qualys/qscan/internal/scanner"
)

type JSONOutput struct {
	Target          string            `json:"target"`
	Type            string            `json:"type"`
	DurationSeconds float64           `json:"duration_seconds"`
	ExitCode        int               `json:"exit_code"`
	Error           string            `json:"error,omitempty"`
	OSInfo          string            `json:"os_info,omitempty"`
	Reports         map[string]string `json:"reports,omitempty"`
}

func PrintJSON(results []*scanner.ScanResult) error {
	var outputs []JSONOutput

	for _, r := range results {
		out := JSONOutput{
			Target:          r.Target,
			Type:            r.Type,
			DurationSeconds: r.DurationSeconds,
			ExitCode:        r.ExitCode,
			Error:           r.Error,
			OSInfo:          r.OSInfo,
			Reports:         r.Reports,
		}
		outputs = append(outputs, out)
	}

	var data []byte
	var err error

	if len(outputs) == 1 {
		data, err = json.MarshalIndent(outputs[0], "", "  ")
	} else {
		data, err = json.MarshalIndent(outputs, "", "  ")
	}

	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	fmt.Println(string(data))
	return nil
}
