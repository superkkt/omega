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

package eas

import (
	"github.com/superkkt/omega/activesync"
	"github.com/superkkt/omega/database"
)

type storage struct {
	dbName string
}

// New returns storage that implements the activesync.Storage interface.
func New(dbName string) *storage {
	return &storage{
		dbName: dbName,
	}
}

// deviceID is case-sensitive.
func (r *storage) NewFolderSync(queryer database.Queryer, userUID uint64, deviceID string) activesync.FolderSync {
	return &folderSync{
		queryer:  queryer,
		userUID:  userUID,
		deviceID: deviceID,
		dbName:   r.dbName,
	}
}

// deviceID is case-sensitive.
func (r *storage) NewSync(queryer database.Queryer, userUID uint64, deviceID string, folderID uint64) activesync.Sync {
	return &synchronize{
		queryer:  queryer,
		userUID:  userUID,
		deviceID: deviceID,
		folderID: folderID,
		dbName:   r.dbName,
	}
}
