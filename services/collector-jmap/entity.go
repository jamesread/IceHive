package main

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"golang.org/x/text/unicode/norm"
)

const entityTypeEmailThread = "EmailThread"

type sourceHash struct {
	HashValue string `json:"hash_value"`
	HashType  string `json:"hash_type"`
}

type collectorMetadata struct {
	EntityType          string     `json:"entity_type"`
	SourceSystem        string     `json:"source_system"`
	SourceCollectorType string     `json:"source_collector_type"`
	SourceUniqueID      string     `json:"source_unique_id"`
	SourceHash          sourceHash `json:"source_hash"`
	ObservedUnixMS      int64      `json:"observed_unix_ms"`
	RecollectSpec       *string    `json:"recollect_spec"`
}

type fieldDescriptor struct {
	Type   string `json:"type"`
	Length int    `json:"length,omitempty"`
}

type entityMessage struct {
	Type          string                     `json:"type"`
	SchemaVersion string                     `json:"schema_version"`
	Metadata      collectorMetadata          `json:"collectormetadata"`
	Structure     map[string]fieldDescriptor `json:"structure"`
	Values        map[string]any             `json:"values"`
}

type emailThreadFields struct {
	ThreadID            string
	MailboxID           string
	AccountID           string
	MessageCount        int64
	Subject             string
	Snippet             string
	LastReceivedUnixMs  int64
}

func buildEmailThreadEntity(row emailThreadFields) entityMessage {
	uniqueID := norm.NFC.String(row.AccountID + "|" + row.ThreadID)
	collector := norm.NFC.String(collectorJmapType)
	sum := sha256.Sum256([]byte(uniqueID + ":" + collector))
	hashValue := hex.EncodeToString(sum[:])
	now := time.Now().UnixMilli()

	structure := map[string]fieldDescriptor{
		"thread_id":              {Type: "string", Length: 512},
		"mailbox_id":           {Type: "string", Length: 512},
		"jmap_account_id":      {Type: "string", Length: 512},
		"message_count":        {Type: "int64"},
		"subject":              {Type: "string", Length: 2048},
		"snippet":              {Type: "string", Length: 8192},
		"last_received_unix_ms": {Type: "int64"},
	}
	values := map[string]any{
		"thread_id":              row.ThreadID,
		"mailbox_id":            row.MailboxID,
		"jmap_account_id":       row.AccountID,
		"message_count":         row.MessageCount,
		"subject":               row.Subject,
		"snippet":               row.Snippet,
		"last_received_unix_ms": row.LastReceivedUnixMs,
	}

	return entityMessage{
		Type:          "Entity",
		SchemaVersion: "v1",
		Metadata: collectorMetadata{
			EntityType:          entityTypeEmailThread,
			SourceSystem:        "jmap",
			SourceCollectorType: collectorJmapType,
			SourceUniqueID:      uniqueID,
			SourceHash: sourceHash{
				HashValue: hashValue,
				HashType:  "sha256",
			},
			ObservedUnixMS: now,
		},
		Structure: structure,
		Values:    values,
	}
}
