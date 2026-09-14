package main

import "strings"

// PowerShell 7 cleans PSModulePath only for directly launched Windows
// PowerShell processes, not when our EXE is between them. Let 5.1 construct
// its own module path. Change only the child environment: preserve PATH,
// proxy settings, execution-policy preferences and all other variables.
func windowsPSEnvironment(environment []string) []string {
	result := make([]string, 0, len(environment))
	for _, item := range environment {
		key, _, _ := strings.Cut(item, "=")
		if !strings.EqualFold(key, "PSModulePath") {
			result = append(result, item)
		}
	}
	return result
}
