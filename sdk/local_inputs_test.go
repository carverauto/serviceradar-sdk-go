//go:build !tinygo

package sdk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadLocalInputsMergesActionAndKeepsCredentialsSeparate(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	configPath := filepath.Join(directory, "config.json")
	actionPath := filepath.Join(directory, "action.json")
	envPath := filepath.Join(directory, ".env")
	mustWriteLocalTestFile(t, configPath, `{"instance_id":"test","page_size":100}`)
	mustWriteLocalTestFile(t, actionPath, `{"schema":"serviceradar.northbound_action_invocation.v1","invocation_id":"run-1","action_id":"collect","input_values":{"type":"Switch"}}`)
	mustWriteLocalTestFile(t, envPath, strings.Join([]string{
		LocalConfigFileVariable + "=" + configPath,
		LocalActionFileVariable + "=" + actionPath,
		LocalCredentialPrefix + "USERNAME=file-user",
		LocalCredentialPrefix + "PASSWORD=file-password",
	}, "\n"))

	inputs, err := LoadLocalInputs(LocalInputOptions{
		EnvFile: envPath,
		Environment: []string{
			LocalCredentialPrefix + "PASSWORD=process-password",
		},
	})
	if err != nil {
		t.Fatalf("LoadLocalInputs() error = %v", err)
	}
	if got, ok := inputs.Credential("username"); !ok || got != "file-user" {
		t.Fatalf("username credential = %q, %v", got, ok)
	}
	if got, ok := inputs.Credential("PASSWORD"); !ok || got != "process-password" {
		t.Fatalf("password credential did not use process override: %q, %v", got, ok)
	}

	runtimeJSON, err := inputs.RuntimeConfigJSON()
	if err != nil {
		t.Fatalf("RuntimeConfigJSON() error = %v", err)
	}
	if strings.Contains(string(runtimeJSON), "file-user") || strings.Contains(string(runtimeJSON), "process-password") {
		t.Fatalf("runtime config leaked a credential: %s", runtimeJSON)
	}
	var runtimeConfig map[string]json.RawMessage
	if err := json.Unmarshal(runtimeJSON, &runtimeConfig); err != nil {
		t.Fatal(err)
	}
	if string(runtimeConfig["page_size"]) != "100" || len(runtimeConfig["action_invocation"]) == 0 {
		t.Fatalf("runtime config did not preserve config/action input: %s", runtimeJSON)
	}
}

func TestLoadLocalInputsRejectsTrailingJSON(t *testing.T) {
	t.Parallel()

	_, err := LoadLocalInputs(LocalInputOptions{
		ConfigJSON:  []byte(`{"ok":true} trailing`),
		Environment: []string{},
	})
	if err == nil {
		t.Fatal("expected trailing config data to fail")
	}
}

func mustWriteLocalTestFile(t *testing.T, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}
