package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/icehive/icehive/services/common/pkg/amqpctl"
	"github.com/icehive/icehive/services/common/pkg/bootstrap"
	"github.com/icehive/icehive/services/common/pkg/persist"
	"github.com/knadh/koanf/v2"
	"github.com/sirupsen/logrus"
)

const defaultDataDir = "./data"

func main() {
	persist.Main(persist.MainConfig{
		ID:            "yaml",
		DefaultListen: ":8083",
		ConfigYAML:    "persister-yaml.yaml",
		Work:          yamlWork,
	})
}

//gocyclo:ignore
func yamlWork(ctx context.Context, k *koanf.Koanf, log *logrus.Logger, boot *bootstrap.WorkerRuntime, amqpClient *amqpctl.Client) error {
	if boot == nil {
		return fmt.Errorf("controller bootstrap settings are required")
	}
	dataDir := defaultDataDir
	if k != nil && k.Exists("data_dir") {
		if v := strings.TrimSpace(k.String("data_dir")); v != "" {
			dataDir = v
		}
	}
	store, err := newYAMLStore(dataDir)
	if err != nil {
		return err
	}

	queueName := amqpctl.QueueName("persister-yaml-entities")
	if err := amqpClient.EnsureQueue(queueName, amqpctl.RoutingKeyCollectorEntities); err != nil {
		return fmt.Errorf("declare entity queue: %w", err)
	}
	log.WithFields(logrus.Fields{
		"exchange":    boot.AMQPExchange,
		"queue":       queueName,
		"routing_key": amqpctl.RoutingKeyCollectorEntities,
		"data_dir":    store.root,
	}).Info("YAML persister consuming entity stream")

	if gitPeriodicEnabled() {
		msg := gitCommitMessage()
		store.runGitSync(ctx, log, msg, "startup")
		go store.runPeriodicGitCommit(ctx, log, msg)
		log.WithFields(logrus.Fields{
			"interval":   gitCommitInterval.String(),
			"data_dir":   store.root,
			"git_commit": gitCommitEnabled(),
			"git_push":   gitPushEnabled(),
		}).Info("periodic git sync enabled (writes paused during commit/push)")
	}

	consumeErr := amqpClient.ConsumeJSON(ctx, queueName, amqpctl.RoutingKeyCollectorEntities, func(hctx context.Context, body []byte) error {
		var msg entityMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			log.WithError(err).WithField("body_len", len(body)).Warn("entity message: JSON decode failed")
			return fmt.Errorf("decode entity json: %w", err)
		}
		path, err := store.writeEntity(hctx, &msg)
		if err != nil {
			log.WithError(err).WithFields(logrus.Fields{
				"entity_type":      msg.Metadata.EntityType,
				"source_unique_id": msg.Metadata.SourceUniqueID,
			}).Error("entity persist failed")
			return err
		}
		log.WithFields(logrus.Fields{
			"entity_type":      msg.Metadata.EntityType,
			"source_unique_id": msg.Metadata.SourceUniqueID,
			"path":             path,
		}).Info("persisted entity write")
		return nil
	})
	if consumeErr != nil && ctx.Err() != nil {
		return nil //nolint:nilerr // context cancellation ends the consumer normally.
	}
	return consumeErr
}
