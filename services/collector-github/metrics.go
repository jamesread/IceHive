package main

import (
	"context"
	"time"

	"github.com/icehive/icehive/services/common/pkg/obsmetrics"
)

const collectorMetricsType = "collector-github"

func observeFetchCtx(ctx context.Context, sourceID, operation string, fn func(context.Context) error) error {
	start := time.Now()
	err := fn(ctx)
	obsmetrics.ObserveCollectorFetchDuration(collectorMetricsType, sourceID, operation, time.Since(start))
	return err
}

func incCollectionRun(sourceID, outcome string) {
	obsmetrics.IncCollectorCollectionRun(collectorMetricsType, sourceID, outcome)
}

func incEntityPublished(entityType string) {
	obsmetrics.IncCollectorEntityPublished(collectorMetricsType, entityType)
}

func incPublishError(entityType string) {
	obsmetrics.IncCollectorPublishError(collectorMetricsType, entityType)
}

func incNormalizeError(entityType, reason string) {
	obsmetrics.IncCollectorNormalizeError(collectorMetricsType, entityType, reason)
}
