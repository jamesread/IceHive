package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/icehive/icehive/services/common/pkg/sourceschema"
	"github.com/icehive/icehive/services/controller/internal/db"
)

type sourceSchemaHeader struct {
	Kind          string `json:"kind"`
	SchemaVersion string `json:"schema_version"`
	CollectorType string `json:"collector_type"`
}

//gocyclo:ignore
func handleCollectorSourceSchemaMessage(ctx context.Context, log *logrus.Logger, sqlDB *sql.DB, body []byte) error {
	var hdr sourceSchemaHeader
	if err := json.Unmarshal(body, &hdr); err != nil {
		log.WithError(err).Debug("source schema message: skip non-json body")
		return nil
	}
	if strings.TrimSpace(hdr.Kind) != sourceschema.Kind {
		return nil
	}
	ct := strings.TrimSpace(hdr.CollectorType)
	sv := strings.TrimSpace(hdr.SchemaVersion)
	if ct == "" || sv == "" {
		log.Warn("source schema message: missing collector_type or schema_version")
		return nil
	}
	if !json.Valid(body) {
		log.Warn("source schema message: invalid JSON")
		return nil
	}
	now := time.Now().UnixMilli()
	if err := db.UpsertCollectorSourceSchema(ctx, sqlDB, ct, sv, body, now); err != nil {
		log.WithError(err).Error("source schema upsert failed")
		return err
	}
	log.WithFields(logrus.Fields{
		"collector_type": ct,
		"schema_version": sv,
	}).Info("stored collector SourceSchema")
	return nil
}
