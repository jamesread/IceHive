package obsmetrics

import (
	"strings"
	"time"
)

// IncPersisterEntityOutcome records an upsert attempt outcome.
func IncPersisterEntityOutcome(persister, entityType, outcome string) {
	persisterEntities.WithLabelValues(persister, entityType, outcome).Inc()
}

// IncPersisterEntityError records a persist failure by error class.
func IncPersisterEntityError(persister, entityType, errorClass string) {
	persisterEntityErrors.WithLabelValues(persister, entityType, errorClass).Inc()
}

// SetPersisterLastSuccess sets the last successful upsert timestamp for an entity type.
func SetPersisterLastSuccess(persister, entityType string, t time.Time) {
	persisterLastSuccess.WithLabelValues(persister, entityType).Set(float64(t.Unix()))
}

// ClassifyPersistError maps a persist error message to a coarse error class for metrics.
func ClassifyPersistError(err error) string {
	if err == nil {
		return "none"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "decode entity json"), strings.Contains(msg, "unsupported entity envelope"):
		return "decode"
	case strings.Contains(msg, "incorrect datetime value"):
		return "datetime_format"
	case strings.Contains(msg, "alter table"), strings.Contains(msg, "create table"), strings.Contains(msg, "empty entity_type"):
		return "schema"
	default:
		return "other"
	}
}
