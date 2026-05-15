package persist

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type memoryStore struct {
	records []Record
}

func (m *memoryStore) Save(_ context.Context, r Record) error {
	m.records = append(m.records, r)
	return nil
}

func (m *memoryStore) Close() error { return nil }

func TestMemoryStoreSave(t *testing.T) {
	var s Store = &memoryStore{}

	err := s.Save(context.Background(), Record{
		Source:  "github",
		Kind:    "commit",
		Payload: []byte(`{"sha":"abc"}`),
	})
	assert.NoError(t, err)
	assert.Len(t, s.(*memoryStore).records, 1)
	assert.Equal(t, "github", s.(*memoryStore).records[0].Source)
}
