package backend

import (
	"time"

	"github.com/Muzikatoshi/omega/backend"
)

type FolderHistory struct {
	id        uint64
	operation backend.FolderOperation
	value     backend.Folder
	timestamp time.Time
}

func (e *FolderHistory) ID() uint64 {
	return e.id
}

func (e *FolderHistory) Operation() backend.FolderOperation {
	return e.operation
}

func (e *FolderHistory) Value() (backend.Folder, error) {
	return e.value, nil
}

func (e *FolderHistory) Timestamp() time.Time {
	return e.timestamp
}
