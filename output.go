package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// writeOutput dispatches to the appropriate formatter based on the given
// format string. Returns an error for unknown formats.
func writeOutput(format string, entries []ImageEntry, writer io.Writer) error {
	switch format {
	case "table":
		return writeTableOutput(entries, writer)
	case "json":
		return writeJSONOutput(entries, writer)
	case "csv":
		return writeCSVOutput(entries, writer)
	case "list":
		return writeListOutput(entries, writer)
	default:
		return fmt.Errorf("unknown output format %q: must be one of: table, json, csv, list", format)
	}
}

// writeTableOutput renders image entries as a human-readable aligned table.
// Columns are: RESOURCE (kind/namespace/name), CHART, CONTAINER, IMAGE, TAG.
func writeTableOutput(entries []ImageEntry, writer io.Writer) error {
	tabWriter := tabwriter.NewWriter(writer, 0, 0, 3, ' ', 0)
	defer tabWriter.Flush()

	headerSeparator := strings.Repeat("-", 24) + "\t" +
		strings.Repeat("-", 20) + "\t" +
		strings.Repeat("-", 18) + "\t" +
		strings.Repeat("-", 45) + "\t" +
		strings.Repeat("-", 20)

	fmt.Fprintln(tabWriter, "RESOURCE\tCHART\tCONTAINER\tIMAGE\tTAG")
	fmt.Fprintln(tabWriter, headerSeparator)

	for _, entry := range entries {
		resourceLabel := entry.Kind + "/" + entry.Namespace + "/" + entry.ResourceName
		fmt.Fprintf(tabWriter, "%s\t%s\t%s\t%s\t%s\n",
			resourceLabel,
			entry.ChartLabel,
			entry.ContainerName,
			entry.Image,
			entry.Tag,
		)
	}

	return nil
}

// writeJSONOutput renders image entries as a pretty-printed JSON array.
// Each entry is a JSON object with all fields from ImageEntry.
func writeJSONOutput(entries []ImageEntry, writer io.Writer) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(entries)
}

// writeListOutput renders image entries as a plain newline-separated list of
// "image:tag" references, one per line. Intended for copy-paste into a Zarf
// package images block or any other tool that wants a bare image list.
func writeListOutput(entries []ImageEntry, writer io.Writer) error {
	for _, entry := range entries {
		fmt.Fprintf(writer, "%s:%s\n", entry.Image, entry.Tag)
	}
	return nil
}

// writeCSVOutput renders image entries as a CSV file with a header row.
// Columns are: resource, chart, container, image, tag.
func writeCSVOutput(entries []ImageEntry, writer io.Writer) error {
	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()

	headerRow := []string{"resource", "chart", "container", "image", "tag"}
	if err := csvWriter.Write(headerRow); err != nil {
		return fmt.Errorf("error writing CSV header: %w", err)
	}

	for _, entry := range entries {
		resourceLabel := entry.Kind + "/" + entry.Namespace + "/" + entry.ResourceName
		dataRow := []string{resourceLabel, entry.ChartLabel, entry.ContainerName, entry.Image, entry.Tag}
		if err := csvWriter.Write(dataRow); err != nil {
			return fmt.Errorf("error writing CSV row: %w", err)
		}
	}

	return csvWriter.Error()
}
