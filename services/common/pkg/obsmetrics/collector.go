package obsmetrics

import "time"

// ObserveCollectorFetchDuration records upstream fetch latency.
func ObserveCollectorFetchDuration(collectorType, sourceID, operation string, d time.Duration) {
	collectorFetchDuration.WithLabelValues(collectorType, sourceID, operation).Observe(d.Seconds())
}

// IncCollectorNormalizeError increments normalization failure counter.
func IncCollectorNormalizeError(collectorType, entityType, reason string) {
	collectorNormalizeErrors.WithLabelValues(collectorType, entityType, reason).Inc()
}

// IncCollectorEntityPublished increments successful entity publish counter.
func IncCollectorEntityPublished(collectorType, entityType string) {
	collectorEntitiesPublished.WithLabelValues(collectorType, entityType).Inc()
}

// IncCollectorPublishError increments AMQP publish failure counter.
func IncCollectorPublishError(collectorType, entityType string) {
	collectorPublishErrors.WithLabelValues(collectorType, entityType).Inc()
}

// IncCollectorCollectionRun records a collection source run outcome (success or failure).
func IncCollectorCollectionRun(collectorType, sourceID, outcome string) {
	collectorCollectionRuns.WithLabelValues(collectorType, sourceID, outcome).Inc()
}
