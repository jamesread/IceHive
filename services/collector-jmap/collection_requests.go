package main

import (
	"context"
	"errors"
	"time"

	"github.com/icehive/icehive/services/common/pkg/amqpctl"
	icehivev1 "github.com/icehive/icehive/services/common/pkg/gen/icehive/v1"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/encoding/protojson"
)

const collectionRequestQueueLogical = "collector-jmap-collection-requests"

var collectionRequestJSON = protojson.UnmarshalOptions{DiscardUnknown: true}

//gocyclo:ignore
func consumeCollectionRequests(ctx context.Context, log *logrus.Logger, amqpClient *amqpctl.Client, controllerBaseURL string) error {
	rk := amqpctl.CollectorCollectionRequestRoutingKey(collectorJmapType)
	q := amqpctl.QueueName(collectionRequestQueueLogical)
	if err := amqpClient.EnsureQueue(q, rk); err != nil {
		return err
	}
	return amqpClient.ConsumeJSON(ctx, q, rk, func(hctx context.Context, body []byte) error {
		msg := &icehivev1.CollectionRequest{}
		if err := collectionRequestJSON.Unmarshal(body, msg); err != nil {
			log.WithError(err).Warn("dropping malformed collection request JSON")
			return nil
		}
		src := msg.GetSource()
		if src == nil {
			return nil
		}
		if src.GetCollectorType() != collectorJmapType {
			return nil
		}
		log.WithFields(logrus.Fields{
			"source_id":   src.GetId(),
			"source_spec": src.GetSourceSpec(),
		}).Info("jmap: collection request received; starting run")
		rt, err := jmapRuntimeFromEnv(hctx)
		if err != nil {
			log.WithError(err).Warn("collection request: JMAP env not configured")
			return err
		}
		logJmapConnectionOK(log, rt)
		runOneJmapSource(hctx, log, amqpClient, controllerBaseURL, rt, src, time.Now().UTC())
		return nil
	})
}

//gocyclo:ignore
func startCollectionRequestConsumer(ctx context.Context, log *logrus.Logger, amqpClient *amqpctl.Client, controllerBaseURL string) {
	go func() {
		for {
			err := consumeCollectionRequests(ctx, log, amqpClient, controllerBaseURL)
			if err == nil || errors.Is(err, context.Canceled) {
				return
			}
			log.WithError(err).Warn("collection request consumer stopped; retrying in 5s")
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}()
}
