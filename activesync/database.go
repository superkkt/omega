/*
 * Omega is an advanced email service that supports Microsoft ActiveSync.
 *
 * Copyright (C) 2016, 2017 Muzi Katoshi <muzikatoshi@gmail.com>
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

package activesync

import (
	"time"

	"github.com/Muzikatoshi/omega/backend"
	"github.com/Muzikatoshi/omega/database"
)

// NOTE: All methods of Storage should return database.TransactionError if an error occurrs.
type Storage interface {
	NewFolderSync(queryer database.Queryer, userUID uint64, deviceID string) FolderSync
	NewSync(queryer database.Queryer, userUID uint64, deviceID string, folderID uint64) Sync
}

type CommonSync interface {
	UserUID() uint64
	DeviceID() string
	ClearSyncKeys() error
	// LoadSyncKey returns a history ID related with syncKey. It is necessary
	// to acquire a read or write lock depending on the lock mode for the
	// syncKey to prevent any concurrent updates from another transaction.
	LoadSyncKey(syncKey uint64, lock database.LockMode) (historyID uint64, err error)
	NewSyncKey(historyID uint64) (syncKey uint64, err error)
	// GetLastSyncKey returns the last issued sync key. It is necessary
	// to acquire a read or write lock depending on the lock mode for the
	// syncKey to prevent any concurrent updates from another transaction.
	GetLastSyncKey(lock database.LockMode) (syncKey uint64, ok bool, err error)
}

type FolderSync interface {
	CommonSync
	ClearVirtualFolders() error
	AddVirtualFolder(folder backend.Folder, lastHistoryID uint64) error
	// GetVirtualFolder returns a virtual folder whose folder ID is folderID.
	// It is necessary to acquire a read or write lock depending on the lock
	// mode for the virtual folder to prevent any concurrent updates from
	// another transaction.
	GetVirtualFolder(folderID uint64, lock database.LockMode) (folder VirtualFolder, err error)
	UpdateVirtualFolder(folder backend.Folder) error
	RemoveVirtualFolder(folderID uint64) error
}

type VirtualFolder struct {
	ID            uint64
	Name          string
	ParentID      uint64
	LastHistoryID uint64
}

type Sync interface {
	CommonSync
	FolderID() uint64
	ClearVirtualEmails() error
	// GetOldestVirtualEmail returns the most oldest email in the virtual
	// folder. It is necessary to acquire a read or write lock depending on
	// the lock mode for the virtual email to prevent any concurrent updates
	// from another transaction.
	GetOldestVirtualEmail(lock database.LockMode) (email VirtualEmail, ok bool, err error)
	AddVirtualEmail(email *backend.Email, lastHistoryID uint64) error
	// GetOldVirtualEmails returns virtual emails whose timestamps are past
	// threshold. It is necessary to acquire a read or write lock depending
	// on the lock mode for the virtual emails to prevent any concurrent
	// updates from another transaction. limit is the maximum number of
	// virtual emails that can be returned. GetOldVirtualEmails can return
	// nil if there is no virtual emails we find.
	GetOldVirtualEmails(threshold time.Time, limit uint, lock database.LockMode) ([]VirtualEmail, error)
	// GetVirtualEmail returns a virtual email whose ID is emailID. It is
	// necessary to acquire a read or write lock depending on the lock mode
	// for the virtual email to prevent any concurrent updates from another
	// transaction.
	GetVirtualEmail(emailID uint64, lock database.LockMode) (email VirtualEmail, err error)
	UpdateVirtualEmailSeen(emailID uint64, seen bool) error
	RemoveVirtualEmail(emailID uint64) error
}

type VirtualEmail struct {
	ID            uint64 // Email ID, not virtual one's ID.
	Seen          bool
	Timestamp     time.Time
	LastHistoryID uint64
}
