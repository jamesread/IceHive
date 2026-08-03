package main

type sourceHash struct {
	HashValue string `json:"hash_value"`
	HashType  string `json:"hash_type"`
}

type collectorMetadata struct {
	RecollectSpec       *string    `json:"recollect_spec"`
	EntityType          string     `json:"entity_type"`
	SourceSystem        string     `json:"source_system"`
	SourceCollectorType string     `json:"source_collector_type"`
	SourceUniqueID      string     `json:"source_unique_id"`
	SourceHash          sourceHash `json:"source_hash"`
	ObservedUnixMS      int64      `json:"observed_unix_ms"`
}

type fieldDescriptor struct {
	Type string `json:"type"`
}

type entityMessage struct {
	Structure     map[string]fieldDescriptor `json:"structure"`
	Values        map[string]any             `json:"values"`
	Type          string                     `json:"type"`
	SchemaVersion string                     `json:"schema_version"`
	Metadata      collectorMetadata          `json:"collectormetadata"`
}

// persistedEntity is written to disk (structure/schema is omitted).
type persistedEntity struct {
	Values        map[string]any    `yaml:"values"`
	Type          string            `yaml:"type"`
	SchemaVersion string            `yaml:"schema_version"`
	Metadata      collectorMetadata `yaml:"collectormetadata"`
}

func toPersistedEntity(msg *entityMessage) persistedEntity {
	return persistedEntity{
		Type:          msg.Type,
		SchemaVersion: msg.SchemaVersion,
		Metadata:      msg.Metadata,
		Values:        msg.Values,
	}
}
