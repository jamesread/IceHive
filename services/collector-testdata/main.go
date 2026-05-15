package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/icehive/icehive/services/common/pkg/amqpctl"
	"github.com/icehive/icehive/services/common/pkg/bootstrap"
	"github.com/icehive/icehive/services/common/pkg/collector"
	"github.com/icehive/icehive/services/common/pkg/sourceschema"
	"github.com/knadh/koanf/v2"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/unicode/norm"
)

const (
	collectorType  = "collector-testdata"
	defaultSeconds = 15
)

type animalSeed struct {
	ID           string
	Name         string
	Species      string
	Family       string
	LegCount     int64
	IsDomestic   bool
	Conservation string
}

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

var animals = []animalSeed{
	{ID: "animal-1", Name: "Luna", Species: "Cat", Family: "Felidae", LegCount: 4, IsDomestic: true, Conservation: "least_concern"},
	{ID: "animal-2", Name: "Rex", Species: "Dog", Family: "Canidae", LegCount: 4, IsDomestic: true, Conservation: "least_concern"},
	{ID: "animal-3", Name: "Kiki", Species: "Parrot", Family: "Psittacidae", LegCount: 2, IsDomestic: false, Conservation: "least_concern"},
}

func main() {
	collector.Main(collector.MainConfig{
		ID:            "testdata",
		DefaultListen: ":8085",
		ConfigYAML:    "collector-testdata.yaml",
		Work:          testDataWork,
	})
}

func testDataWork(ctx context.Context, k *koanf.Koanf, log *logrus.Logger, boot *bootstrap.WorkerRuntime, amqpClient *amqpctl.Client, _ string) error {
	if boot != nil {
		log.WithFields(logrus.Fields{
			"exchange":    boot.AMQPExchange,
			"routing_key": amqpctl.RoutingKeyCollectorEntities,
		}).Info("Testdata collector emitting Animal entities")
	}

	interval := time.Duration(defaultSeconds) * time.Second
	if k != nil && k.Exists("emit_interval_seconds") {
		sec := k.Int("emit_interval_seconds")
		if sec > 0 {
			interval = time.Duration(sec) * time.Second
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if amqpClient != nil {
		if err := sourceschema.Publish(ctx, amqpClient, sourceschema.TestdataV1()); err != nil {
			log.WithError(err).Warn("publish SourceSchema failed")
		}
	}

	if err := emitBatch(ctx, amqpClient, log); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := emitBatch(ctx, amqpClient, log); err != nil {
				log.WithError(err).Warn("failed to emit testdata entity batch")
			}
		}
	}
}

func emitBatch(ctx context.Context, client *amqpctl.Client, log *logrus.Logger) error {
	for _, animal := range animals {
		msg := buildAnimalEntity(animal)
		payload, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("marshal entity %s: %w", animal.ID, err)
		}
		if err := client.PublishJSON(ctx, amqpctl.RoutingKeyCollectorEntities, payload); err != nil {
			return fmt.Errorf("publish entity %s: %w", animal.ID, err)
		}
		log.WithFields(logrus.Fields{
			"entity_type":      msg.Metadata.EntityType,
			"source_unique_id": msg.Metadata.SourceUniqueID,
			"routing_key":      amqpctl.RoutingKeyCollectorEntities,
		}).Info("emitted testdata entity")
	}
	return nil
}

func buildAnimalEntity(animal animalSeed) entityMessage {
	uniqueID := norm.NFC.String(animal.ID)
	collector := norm.NFC.String(collectorType)
	sum := sha256.Sum256([]byte(uniqueID + ":" + collector))
	hashValue := hex.EncodeToString(sum[:])
	now := time.Now().UnixMilli()

	return entityMessage{
		Type:          "Entity",
		SchemaVersion: "v1",
		Metadata: collectorMetadata{
			EntityType:          "Animal",
			SourceSystem:        "testdata",
			SourceCollectorType: collectorType,
			SourceUniqueID:      uniqueID,
			SourceHash: sourceHash{
				HashValue: hashValue,
				HashType:  "sha256",
			},
			ObservedUnixMS: now,
		},
		Structure: map[string]fieldDescriptor{
			"name":                {Type: "string", Length: 255},
			"species":             {Type: "string", Length: 255},
			"family":              {Type: "string", Length: 255},
			"leg_count":           {Type: "int64"},
			"is_domestic":         {Type: "bool"},
			"conservation_status": {Type: "string", Length: 64},
		},
		Values: map[string]any{
			"name":                animal.Name,
			"species":             animal.Species,
			"family":              animal.Family,
			"leg_count":           animal.LegCount,
			"is_domestic":         animal.IsDomestic,
			"conservation_status": animal.Conservation,
		},
	}
}
