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

const collectionRequestQueueLogical = "collector-rss-collection-requests"

var collectionRequestJSON = protojson.UnmarshalOptions{DiscardUnknown: true}

func consumeCollectionRequests(ctx context.Context, log *logrus.Logger, amqpClient *amqpctl.Client, controllerBaseURL string, fetchTimeout time.Duration, userAgent string, itemsMax int) error {
	rk := amqpctl.CollectorCollectionRequestRoutingKey(collectorRssType)
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
		if src.GetCollectorType() != collectorRssType {
			return nil
		}
		log.WithFields(logrus.Fields{
			"source_id":   src.GetId(),
			"source_spec": src.GetSourceSpec(),
		}).Info("rss: collection request received; starting run")
		runOneRssSource(hctx, log, amqpClient, controllerBaseURL, src, time.Now().UTC(), fetchTimeout, userAgent, itemsMax)
		return nil
	})
}

func startCollectionRequestConsumer(ctx context.Context, log *logrus.Logger, amqpClient *amqpctl.Client, controllerBaseURL string, fetchTimeout time.Duration, userAgent string, itemsMax int) {
	go func() {
		for {
			err := consumeCollectionRequests(ctx, log, amqpClient, controllerBaseURL, fetchTimeout, userAgent, itemsMax)
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
