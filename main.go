package main

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
)

// Config holds all user-provided options parsed from command-line flags.
type Config struct {
	ReleaseName  string
	ChartPath    string
	ValuesFiles  []string
	SetValues    []string
	OutputFormat string
	UniqueOnly   bool
	Namespace    string
}


func main() {
	config, err := parseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	renderedChart, err := runHelmTemplate(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error running helm template: %v\n", err)
		os.Exit(1)
	}

	imageEntries, err := extractImagesFromRenderedChart(renderedChart)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing rendered chart: %v\n", err)
		os.Exit(1)
	}

	if config.UniqueOnly {
		imageEntries = deduplicateByImageAndTag(imageEntries)
	}

	if err := writeOutput(config.OutputFormat, imageEntries, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error writing output: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() (Config, error) {
	// pflag supports interspersed flags, meaning flags can appear before or
	// after positional arguments (unlike the standard library flag package).
	flagSet := pflag.NewFlagSet("helm-image-finder", pflag.ContinueOnError)

	valuesFiles := flagSet.StringArrayP("values", "f", nil, "path to a values file; may be repeated")
	setValues := flagSet.StringArray("set", nil, "override a chart value (helm --set syntax); may be repeated")
	outputFormat := flagSet.StringP("output", "o", "table", "output format: table, json, csv, list")
	uniqueOnly := flagSet.BoolP("unique", "u", false, "only print unique image+tag combinations (first occurrence wins)")
	namespace := flagSet.StringP("namespace", "n", "", "kubernetes namespace to pass to helm template")

	flagSet.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: helm-image-finder [flags] <release-name> <chart>\n\n")
		fmt.Fprintf(os.Stderr, "Extracts all container images from a rendered Helm chart.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flagSet.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  helm-image-finder my-release ./chart\n")
		fmt.Fprintf(os.Stderr, "  helm-image-finder my-release ./chart -f prod.yaml --set image.tag=1.2.3\n")
		fmt.Fprintf(os.Stderr, "  helm-image-finder my-release ./chart --output json --unique\n")
		fmt.Fprintf(os.Stderr, "  helm-image-finder my-release ./chart --output list --unique\n")
		fmt.Fprintf(os.Stderr, "  helm-image-finder my-release oci://registry.io/org/chart --version 1.5.0\n")
	}

	if err := flagSet.Parse(os.Args[1:]); err != nil {
		return Config{}, err
	}

	remainingArgs := flagSet.Args()
	if len(remainingArgs) < 2 {
		flagSet.Usage()
		return Config{}, fmt.Errorf("release name and chart are required")
	}

	validOutputFormats := map[string]bool{"table": true, "json": true, "csv": true, "list": true}
	if !validOutputFormats[*outputFormat] {
		return Config{}, fmt.Errorf("unknown output format %q: must be one of: table, json, csv", *outputFormat)
	}

	return Config{
		ReleaseName:  remainingArgs[0],
		ChartPath:    remainingArgs[1],
		ValuesFiles:  derefStringSlice(valuesFiles),
		SetValues:    derefStringSlice(setValues),
		OutputFormat: *outputFormat,
		UniqueOnly:   *uniqueOnly,
		Namespace:    *namespace,
	}, nil
}

func derefStringSlice(pointer *[]string) []string {
	if pointer == nil {
		return nil
	}
	return *pointer
}
