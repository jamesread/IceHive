package obsmetrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Register ensures all IceHive domain metrics are registered with the default registry.
func Register() {
	// Metrics are registered via promauto at package init; this hook exists so
	// worker main functions can document the intentional registration point.
	_ = collectorFetchDuration
}

var (
	collectorFetchDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "icehive_collector_fetch_duration_seconds",
		Help:    "Duration of upstream fetch operations in collectors.",
		Buckets: prometheus.DefBuckets,
	}, []string{"collector_type", "source_id", "operation"})

	collectorNormalizeErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "icehive_collector_normalize_errors_total",
		Help: "Entity normalization failures in collectors.",
	}, []string{"collector_type", "entity_type", "reason"})

	collectorEntitiesPublished = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "icehive_collector_entities_published_total",
		Help: "Entities successfully published to AMQP by collectors.",
	}, []string{"collector_type", "entity_type"})

	collectorPublishErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "icehive_collector_publish_errors_total",
		Help: "AMQP publish failures for entity messages.",
	}, []string{"collector_type", "entity_type"})

	collectorCollectionRuns = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "icehive_collector_collection_runs_total",
		Help: "Collection source run outcomes reported by collectors.",
	}, []string{"collector_type", "source_id", "outcome"})

	persisterEntities = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "icehive_persister_entities_total",
		Help: "Entity upsert attempts in persisters.",
	}, []string{"persister", "entity_type", "outcome"})

	persisterEntityErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "icehive_persister_entity_errors_total",
		Help: "Entity persist failures in persisters by error class.",
	}, []string{"persister", "entity_type", "error_class"})

	persisterLastSuccess = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "icehive_persister_last_success_timestamp_seconds",
		Help: "Unix timestamp of the last successful entity upsert per entity type.",
	}, []string{"persister", "entity_type"})

	controllerServiceHeartbeatAge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "icehive_controller_service_heartbeat_age_seconds",
		Help: "Seconds since the latest AMQP heartbeat for each service.",
	}, []string{"service_name"})

	controllerCollectionSourceLastSuccessAge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "icehive_controller_collection_source_last_success_age_seconds",
		Help: "Seconds since the last successful collection run for each source.",
	}, []string{"source_id", "collector_type"})

	controllerCollectionSourceStale = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "icehive_controller_collection_source_stale",
		Help: "1 when a collection source is overdue or never succeeded; 0 otherwise.",
	}, []string{"source_id", "collector_type"})

	controllerEntityFreshnessAge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "icehive_controller_entity_freshness_age_seconds",
		Help: "Seconds since the newest row update in the entity sink for each entity type.",
	}, []string{"entity_type", "source_system"})
)
