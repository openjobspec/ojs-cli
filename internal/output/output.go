package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

// Format controls the output format ("table" or "json").
var Format = "table"

// JSON prints data as formatted JSON.
func JSON(data any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// Table prints rows in a table format with headers.
func Table(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	fmt.Fprintln(w, strings.Repeat("─", len(headers)*16))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	w.Flush()
}

// PrintResult prints data in the configured format.
func PrintResult(data any, headers []string, toRow func(any) []string) error {
	if Format == "json" {
		return JSON(data)
	}

	switch v := data.(type) {
	case []any:
		rows := make([][]string, 0, len(v))
		for _, item := range v {
			rows = append(rows, toRow(item))
		}
		Table(headers, rows)
	default:
		return JSON(data)
	}
	return nil
}

// Success prints a success message.
func Success(format string, args ...any) {
	fmt.Printf("✓ "+format+"\n", args...)
}

// Warn prints a warning message.
func Warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "⚠ "+format+"\n", args...)
}
