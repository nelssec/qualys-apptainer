package output

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/olekukonko/tablewriter"
	"github.com/qualys/qscan/internal/scanner"
)

func PrintTable(result *scanner.ScanResult) {
	fmt.Println()
	fmt.Println("Qualys QScanner Results")
	fmt.Println("=======================")
	fmt.Printf("Target:     %s\n", result.Target)
	fmt.Printf("Type:       %s\n", result.Type)
	if result.OSInfo != "" {
		fmt.Printf("OS:         %s\n", result.OSInfo)
	}
	fmt.Printf("Duration:   %.1fs\n", result.DurationSeconds)
	fmt.Printf("Exit Code:  %d\n", result.ExitCode)

	if result.Error != "" {
		fmt.Printf("Error:      %s\n", result.Error)
	}

	if len(result.Reports) > 0 {
		fmt.Println()
		fmt.Println("Reports:")

		var formats []string
		for format := range result.Reports {
			formats = append(formats, format)
		}
		sort.Strings(formats)

		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Format", "Path"})
		table.SetBorder(false)
		table.SetAutoWrapText(false)

		for _, format := range formats {
			path := result.Reports[format]
			relPath, err := filepath.Rel(".", path)
			if err != nil {
				relPath = path
			}
			table.Append([]string{format, relPath})
		}

		table.Render()
	}

	fmt.Println()
}
