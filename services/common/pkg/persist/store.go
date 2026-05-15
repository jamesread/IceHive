package persist

import "context"

// Record is a normalized data item received from the message queue.
type Record struct {
	Source  string
	Kind    string
	Payload []byte
}

// Store abstracts sink-specific persistence (filesystem, SQL, etc.).
type Store interface {
	Save(ctx context.Context, record Record) error
	Close() error
}
