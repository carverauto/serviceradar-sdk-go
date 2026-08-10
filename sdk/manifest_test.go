package sdk

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestPluginManifestSerializesSDKGeneratedSecurityFixture(t *testing.T) {
	payload, err := securityFixtureManifest().Serialize()
	if err != nil {
		t.Fatalf("serialize manifest: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("decode generated manifest: %v", err)
	}

	expectedRaw, err := os.ReadFile("../testdata/sdk_generated_security_manifest.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var expected map[string]any
	if err := json.Unmarshal(expectedRaw, &expected); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	if !reflect.DeepEqual(got, expected) {
		gotJSON, _ := json.MarshalIndent(got, "", "  ")
		expectedJSON, _ := json.MarshalIndent(expected, "", "  ")
		t.Fatalf("generated manifest mismatch\nwant:\n%s\n\ngot:\n%s", expectedJSON, gotJSON)
	}
}

// The manifest surface is closed on purpose: core rejects any signal_schemas
// key outside its allowlist. A field added here without a matching change in
// core would produce manifests that fail at upload, so pin the encoded key set.
func TestSignalSchemaContributionEncodesOnlyCoreAllowedKeys(t *testing.T) {
	allowed := map[string]bool{
		"id":                       true,
		"version":                  true,
		"signal_type":              true,
		"payload_kind":             true,
		"payload_schema":           true,
		"display_contract":         true,
		"display_contract_id":      true,
		"display_contract_version": true,
		"ocsf_schema_version":      true,
		"class_uid":                true,
		"type_uid":                 true,
	}

	payload, err := json.Marshal(securityFixtureManifest().SignalSchemas[0])
	if err != nil {
		t.Fatalf("marshal contribution: %v", err)
	}

	var encoded map[string]any
	if err := json.Unmarshal(payload, &encoded); err != nil {
		t.Fatalf("decode contribution: %v", err)
	}

	for key := range encoded {
		if !allowed[key] {
			t.Errorf("signal schema encodes %q, which core rejects as not allowed", key)
		}
	}

	for key := range allowed {
		if _, ok := encoded[key]; !ok {
			t.Errorf("fixture does not exercise allowed key %q", key)
		}
	}
}

func TestPluginManifestRejectsPayloadKindCoreDoesNotAccept(t *testing.T) {
	manifest := securityFixtureManifest()
	manifest.SignalSchemas[0].PayloadKind = "json"

	err := manifest.Validate()
	if err == nil {
		t.Fatal("expected unsupported payload_kind to fail validation")
	}

	if !strings.Contains(err.Error(), "payload_kind") {
		t.Fatalf("expected payload_kind error, got %v", err)
	}
}

func TestPluginManifestRejectsUnsupportedCapability(t *testing.T) {
	manifest := securityFixtureManifest()
	manifest.Capabilities = append(manifest.Capabilities, "exec_shell")

	err := manifest.Validate()
	if err == nil {
		t.Fatal("expected unsupported capability to fail validation")
	}

	if !strings.Contains(err.Error(), "exec_shell") {
		t.Fatalf("expected capability error, got %v", err)
	}
}

func TestPluginManifestRejectsTraversingBundlePath(t *testing.T) {
	manifest := securityFixtureManifest()
	manifest.SignalSchemas[0].PayloadSchema = "../../etc/passwd.json"

	err := manifest.Validate()
	if err == nil {
		t.Fatal("expected traversing payload_schema to fail validation")
	}

	if !strings.Contains(err.Error(), "traverse") {
		t.Fatalf("expected traversal error, got %v", err)
	}
}

func TestPluginManifestRejectsNonSemverVersions(t *testing.T) {
	manifest := securityFixtureManifest()
	manifest.SignalSchemas[0].Version = "v1"

	err := manifest.Validate()
	if err == nil {
		t.Fatal("expected non-semver version to fail validation")
	}

	if !strings.Contains(err.Error(), "semver") {
		t.Fatalf("expected semver error, got %v", err)
	}
}

func TestPluginManifestRejectsUnsupportedOutputs(t *testing.T) {
	manifest := securityFixtureManifest()
	manifest.Outputs = "serviceradar.anything.v1"

	err := manifest.Validate()
	if err == nil {
		t.Fatal("expected unsupported outputs to fail validation")
	}

	if !strings.Contains(err.Error(), "outputs") {
		t.Fatalf("expected outputs error, got %v", err)
	}
}

// The semver and bundle-path rules are hand-written rather than compiled
// regexps, so pin them against the patterns core enforces.
func TestSemverRuleMatchesCorePattern(t *testing.T) {
	for _, valid := range []string{"1.0.0", "1.9.0-dev", "0.0.1", "2.3.4+build.5"} {
		manifest := securityFixtureManifest()
		manifest.SignalSchemas[0].Version = valid
		manifest.SignalSchemas[0].DisplayContractVersion = valid

		if err := manifest.Validate(); err != nil {
			t.Errorf("%q should be accepted as semver, got %v", valid, err)
		}
	}

	for _, invalid := range []string{"1.0", "1.0.0.0", "v1.0.0", "1.0.0-", "1.a.0", ""} {
		manifest := securityFixtureManifest()
		manifest.SignalSchemas[0].Version = invalid

		if err := manifest.Validate(); err == nil {
			t.Errorf("%q should be rejected as semver", invalid)
		}
	}
}

func TestBundlePathRuleMatchesCorePattern(t *testing.T) {
	for _, valid := range []string{
		"schemas/scan_activity.schema.json",
		"a.json",
		"a/b/c-d_e.json",
	} {
		manifest := securityFixtureManifest()
		manifest.SignalSchemas[0].PayloadSchema = valid

		if err := manifest.Validate(); err != nil {
			t.Errorf("%q should be accepted as a bundle path, got %v", valid, err)
		}
	}

	for _, invalid := range []string{
		"/absolute/path.json",
		"schemas/../secret.json",
		"schemas/scan_activity.yaml",
		"schemas//double.json",
		".hidden.json",
		"",
	} {
		manifest := securityFixtureManifest()
		manifest.SignalSchemas[0].PayloadSchema = invalid

		if err := manifest.Validate(); err == nil {
			t.Errorf("%q should be rejected as a bundle path", invalid)
		}
	}
}

func TestSignalRefRuleMatchesCorePattern(t *testing.T) {
	for _, invalid := range []string{"Uppercase.id", ".leading.dot", "-leading-dash", "has space", ""} {
		manifest := securityFixtureManifest()
		manifest.SignalSchemas[0].ID = invalid

		if err := manifest.Validate(); err == nil {
			t.Errorf("%q should be rejected as a signal ref", invalid)
		}
	}
}

func TestSerializeRefusesInvalidManifest(t *testing.T) {
	manifest := securityFixtureManifest()
	manifest.ID = ""

	if _, err := manifest.Serialize(); err == nil {
		t.Fatal("expected Serialize to refuse an invalid manifest")
	}
}

func TestNewSignalSchemaContributionDerivesBundlePaths(t *testing.T) {
	contribution := NewSignalSchemaContribution(
		"com.carverauto.security.scan_activity",
		"1.0.0",
		SignalSchemaSignalTypeEvent,
		SignalSchemaPayloadKindOCSFEvent,
	)

	if contribution.PayloadSchema != "schemas/scan_activity.schema.json" {
		t.Errorf("unexpected payload schema path: %s", contribution.PayloadSchema)
	}

	if contribution.DisplayContract != "display/scan_activity.display.json" {
		t.Errorf("unexpected display contract path: %s", contribution.DisplayContract)
	}

	if contribution.DisplayContractID != "com.carverauto.security.scan_activity.display" {
		t.Errorf("unexpected display contract id: %s", contribution.DisplayContractID)
	}

	if contribution.DisplayContractVersion != "1.0.0" {
		t.Errorf("unexpected display contract version: %s", contribution.DisplayContractVersion)
	}
}

func securityFixtureManifest() PluginManifest {
	schema := NewSignalSchemaContribution(
		"com.carverauto.security.scan_activity",
		"1.0.0",
		SignalSchemaSignalTypeEvent,
		SignalSchemaPayloadKindOCSFEvent,
	).WithOCSF("1.9.0-dev", 6007, 600701)

	return PluginManifest{
		ID:           "security-sample",
		Name:         "Security Sample",
		Version:      "1.0.0",
		Entrypoint:   "run_check",
		Runtime:      RuntimeWASIPreview1,
		Capabilities: []string{"get_config", "log", "submit_result", "emit_telemetry"},
		Permissions: map[string]any{
			"allowed_domains": []string{},
			"allowed_ports":   []int{},
		},
		Resources: map[string]any{
			"requested_memory_mb":  32,
			"requested_cpu_ms":     5000,
			"max_open_connections": 4,
		},
		Outputs:       OutputsPluginResult,
		SignalSchemas: []SignalSchemaContribution{schema},
	}
}
