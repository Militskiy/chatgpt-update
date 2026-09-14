package main

import (
	"reflect"
	"testing"
)

func TestWindowsPSChildEnvironment(t *testing.T) {
	input := []string{"Path=C:\\Windows", "PSModulePath=C:\\Program Files\\PowerShell\\7\\Modules", "psmodulepath=core-only", "HTTPS_PROXY=http://proxy:8080", "PSExecutionPolicyPreference=AllSigned", "CUSTOM=preserved", "=C:=C:\\test"}
	original := append([]string(nil), input...)
	expected := []string{input[0], input[3], input[4], input[5], input[6]}
	result := windowsPSEnvironment(input)
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("child env: %v", result)
	}
	if !reflect.DeepEqual(input, original) {
		t.Fatal("parent environment was mutated")
	}
}
