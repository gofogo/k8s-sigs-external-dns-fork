/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"sigs.k8s.io/external-dns/pkg/metrics"
)

// ColumnWidths holds the maximum column widths for a metrics markdown table.
type ColumnWidths struct {
	Name      int
	Type      int
	Subsystem int
	Help      int
}

// ComputeColumnWidths returns the minimum column widths needed to align a metrics table,
// seeded with the header label widths.
func ComputeColumnWidths(ms []*metrics.Metric) ColumnWidths {
	w := ColumnWidths{
		Name:      len("Name"),
		Type:      len("Metric Type"),
		Subsystem: len("Subsystem"),
		Help:      len("Help"),
	}
	for _, m := range ms {
		if n := len(m.Name); n > w.Name {
			w.Name = n
		}
		if n := len(m.Type); n > w.Type {
			w.Type = n
		}
		if n := len(m.Subsystem); n > w.Subsystem {
			w.Subsystem = n
		}
		if n := len(m.Help); n > w.Help {
			w.Help = n
		}
	}
	return w
}

// ComputeRuntimeWidth returns the minimum column width needed to align a single-column
// runtime metrics table, seeded with the "Name" header width.
func ComputeRuntimeWidth(ms []string) int {
	w := len("Name")
	for _, m := range ms {
		if n := len(m); n > w {
			w = n
		}
	}
	return w
}

func WriteToFile(filename string, content string) error {
	file, fileErr := os.Create(filename)
	if fileErr != nil {
		_ = fmt.Errorf("failed to create file: %w", fileErr)
	}
	defer file.Close()
	_, writeErr := file.WriteString(content)
	if writeErr != nil {
		_ = fmt.Errorf("failed to write to file: %s", filename)
	}
	return nil
}

// FuncMap returns a mapping of all of the functions that Engine has.
func FuncMap() template.FuncMap {
	return template.FuncMap{
		"backtick": func(times int) string {
			return strings.Repeat("`", times)
		},
		"capitalize": cases.Title(language.English, cases.Compact).String,
		"replace":    strings.ReplaceAll,
		"lower":      strings.ToLower,
		// padRight pads s with spaces on the right to the given width.
		"padRight": func(width int, s string) string {
			return fmt.Sprintf("%-*s", width, s)
		},
		// leftSep generates a left-aligned markdown table separator of the given column width.
		"leftSep": func(width int) string {
			return strings.Repeat("-", width+1)
		},
	}
}
