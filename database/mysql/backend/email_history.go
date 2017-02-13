package backend

import (
	"time"

	"github.com/Muzikatoshi/omega/backend"
)

type EmailHistory struct {
	id        uint64
	operation backend.EmailOperation
	value     backend.Email
	timestamp time.Time
}

func (e *EmailHistory) ID() uint64 {
	return e.id
}

func (e *EmailHistory) Operation() backend.EmailOperation {
	return e.operation
}

func (e *EmailHistory) Value() (*backend.Email, error) {
	return &e.value, nil
}

func (e *EmailHistory) Timestamp() time.Time {
	return e.timestamp
}
