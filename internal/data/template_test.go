package data

import (
	"testing"
)

func TestExtractEnvKeyOrder(t *testing.T) {
	yamlData := []byte(`
name:
  en: "Test"
environments:
  DB_TYPE:
    description:
      en: "Database Type"
    type: "select"
  DB_HOST:
    description:
      en: "Database Host"
    type: "text"
  DB_NAME:
    description:
      en: "Database Name"
    type: "text"
  DB_USER:
    description:
      en: "Database User"
    type: "text"
  DB_PASS:
    description:
      en: "Database Password"
    type: "password"
  WEB_PORT:
    description:
      en: "Web Port"
    type: "port"
  SSH_PORT:
    description:
      en: "SSH Port"
    type: "port"
`)

	keys := extractEnvKeyOrder(yamlData, "environments")
	expected := []string{"DB_TYPE", "DB_HOST", "DB_NAME", "DB_USER", "DB_PASS", "WEB_PORT", "SSH_PORT"}

	if len(keys) != len(expected) {
		t.Fatalf("expected %d keys, got %d", len(expected), len(keys))
	}
	for i, key := range keys {
		if key != expected[i] {
			t.Errorf("key[%d]: expected %q, got %q", i, expected[i], key)
		}
	}
}

func TestExtractEnvKeyOrder_NoEnvironments(t *testing.T) {
	yamlData := []byte(`
name:
  en: "Test"
`)

	keys := extractEnvKeyOrder(yamlData, "environments")
	if keys != nil {
		t.Errorf("expected nil, got %v", keys)
	}
}

func TestExtractEnvKeyOrder_InvalidYAML(t *testing.T) {
	keys := extractEnvKeyOrder([]byte(`{invalid`), "environments")
	if keys != nil {
		t.Errorf("expected nil, got %v", keys)
	}
}
