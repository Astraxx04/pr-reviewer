package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
)

// printJSON pretty-prints raw JSON bytes to stdout. Used for the --json output mode.
func printJSON(data []byte) error {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		// Not JSON (e.g. CSV/plain) — emit verbatim.
		_, err = os.Stdout.Write(data)
		return err
	}
	_, err := fmt.Fprintln(os.Stdout, buf.String())
	return err
}

// table renders aligned rows under the given headers using tab stops.
type table struct {
	w    *tabwriter.Writer
	cols int
}

func newTable(headers ...string) *table {
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	t := &table{w: tw, cols: len(headers)}
	t.row(headers...)
	return t
}

func (t *table) row(cells ...string) {
	_, _ = fmt.Fprintln(t.w, joinTab(cells))
}

func (t *table) flush() { _ = t.w.Flush() }

func joinTab(cells []string) string {
	var b bytes.Buffer
	for i, c := range cells {
		if i > 0 {
			b.WriteByte('\t')
		}
		b.WriteString(c)
	}
	return b.String()
}

// writeOut writes data to the given path, or to stdout when path is empty.
func writeOut(path string, data []byte) error {
	if path == "" {
		_, err := os.Stdout.Write(data)
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Wrote %d bytes to %s\n", len(data), path)
	return nil
}
