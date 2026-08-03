package main

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestEntityFilePath(t *testing.T) {
	path, err := entityFilePath("/data", "GitRepo", "repo:icehive/icehive", "abc123")
	if err != nil {
		t.Fatal(err)
	}
	want := "/data/GitRepo/repo_icehive_icehive.yaml"
	if path != want {
		t.Fatalf("got %q want %q", path, want)
	}
}

func TestEntityFileNameFromHash(t *testing.T) {
	name, err := entityFileName("", "deadbeef")
	if err != nil {
		t.Fatal(err)
	}
	if name != "deadbeef.yaml" {
		t.Fatalf("got %q", name)
	}
}

//gocyclo:ignore
func TestToPersistedEntityOmitsStructure(t *testing.T) {
	msg := &entityMessage{
		Type:          "Entity",
		SchemaVersion: "v1",
		Metadata: collectorMetadata{
			EntityType:     "Animal",
			SourceUniqueID: "dog-1",
		},
		Structure: map[string]fieldDescriptor{"name": {Type: "string"}},
		Values:    map[string]any{"name": "Rex"},
	}
	out := toPersistedEntity(msg)
	if out.Values["name"] != "Rex" {
		t.Fatalf("values: %+v", out.Values)
	}
	body, err := yaml.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if strings.Contains(s, "structure:") {
		t.Fatalf("yaml must not contain structure: %s", s)
	}
	for _, key := range []string{"type:", "schema_version:", "collectormetadata:", "values:"} {
		if !strings.Contains(s, key) {
			t.Fatalf("yaml missing %q: %s", key, s)
		}
	}
}
