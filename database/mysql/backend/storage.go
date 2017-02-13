package backend

import (
	"github.com/Muzikatoshi/omega/backend"
	"github.com/Muzikatoshi/omega/database"
)

type storage struct {
	dbName string
}

// New returns storage that implements the backend.Storage interface.
func New(dbName string) *storage {
	return &storage{
		dbName: dbName,
	}
}

func (r *storage) NewEmailManager(queryer database.Queryer, c backend.Credential, folderID uint64) backend.EmailManager {
	return &EmailStorage{
		c:        c,
		folderID: folderID,
		queryer:  queryer,
		dbName:   r.dbName,
	}
}

func (r *storage) NewFolderManager(queryer database.Queryer, c backend.Credential) backend.FolderManager {
	return &FolderStorage{
		c:       c,
		queryer: queryer,
		dbName:  r.dbName,
	}
}
