/*
 * Omega is an advanced email service that supports Microsoft ActiveSync.
 *
 * Copyright (C) 2016, 2017 Kitae Kim <superkkt@gmail.com>
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License along
 * with this program; if not, write to the Free Software Foundation, Inc.,
 * 51 Franklin Street, Fifth Floor, Boston, MA 02110-1301 USA.
 */

package backend

import (
	"github.com/superkkt/omega/backend"
	"github.com/superkkt/omega/database"
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
