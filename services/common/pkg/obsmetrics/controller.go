package obsmetrics

// SetControllerServiceHeartbeatAge sets seconds since the latest heartbeat for a service.
func SetControllerServiceHeartbeatAge(serviceName string, ageSeconds float64) {
	controllerServiceHeartbeatAge.WithLabelValues(serviceName).Set(ageSeconds)
}

// SetControllerCollectionSourceLastSuccessAge sets seconds since last successful collection run.
func SetControllerCollectionSourceLastSuccessAge(sourceID, collectorType string, ageSeconds float64) {
	controllerCollectionSourceLastSuccessAge.WithLabelValues(sourceID, collectorType).Set(ageSeconds)
}

// SetControllerCollectionSourceStale marks whether a collection source is stale (1) or healthy (0).
func SetControllerCollectionSourceStale(sourceID, collectorType string, stale bool) {
	v := 0.0
	if stale {
		v = 1
	}
	controllerCollectionSourceStale.WithLabelValues(sourceID, collectorType).Set(v)
}

// SetControllerEntityFreshnessAge sets seconds since the newest entity row update.
func SetControllerEntityFreshnessAge(entityType, sourceSystem string, ageSeconds float64) {
	controllerEntityFreshnessAge.WithLabelValues(entityType, sourceSystem).Set(ageSeconds)
}
