package main

import (
	"fmt"
	"os/exec"
)

// runHelmTemplate invokes `helm template` with the given config and returns
// the full rendered YAML as a string. All values files and --set overrides
// from the config are forwarded to helm unchanged.
func runHelmTemplate(config Config) (string, error) {
	helmArguments := []string{"template", config.ReleaseName, config.ChartPath}

	for _, valuesFile := range config.ValuesFiles {
		helmArguments = append(helmArguments, "--values", valuesFile)
	}

	for _, setValue := range config.SetValues {
		helmArguments = append(helmArguments, "--set", setValue)
	}

	if config.Namespace != "" {
		helmArguments = append(helmArguments, "--namespace", config.Namespace)
	}

	outputBytes, err := exec.Command("helm", helmArguments...).Output()
	if err != nil {
		exitError, isExitError := err.(*exec.ExitError)
		if isExitError {
			return "", fmt.Errorf("helm template failed:\n%s", string(exitError.Stderr))
		}
		return "", fmt.Errorf("helm template failed: %w", err)
	}

	return string(outputBytes), nil
}
