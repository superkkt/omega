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

package eas

import (
	"database/sql"

	"github.com/Muzikatoshi/omega/activesync"
	"github.com/Muzikatoshi/omega/backend"
	"github.com/Muzikatoshi/omega/database"
	"github.com/Muzikatoshi/omega/database/mysql"
)

type folderSync struct {
	queryer  database.Queryer
	userUID  uint64
	deviceID string
	dbName   string
}

func (r *folderSync) UserUID() uint64 {
	return r.userUID
}

func (r *folderSync) DeviceID() string {
	return r.deviceID
}

func (r *folderSync) LoadSyncKey(syncKey uint64, lock database.LockMode) (historyID uint64, err error) {
	f := func(tx *sql.Tx) error {
		qry := "SELECT `history_id` FROM `" + r.dbName + "`.`folder_synckey` "
		qry += "WHERE `id` = ? AND `user_uid` = ? AND `device_id` = ? "
		qry += mysql.GetLockCmd(lock)
		if err := tx.QueryRow(qry, syncKey, r.userUID, r.deviceID).Scan(&historyID); err != nil {
			return err
		}

		return nil
	}
	if err := r.queryer.Query(f); err != nil {
		return 0, err
	}

	return historyID, nil
}

func (r *folderSync) NewSyncKey(historyID uint64) (syncKey uint64, err error) {
	f := func(tx *sql.Tx) error {
		qry := "INSERT INTO `" + r.dbName + "`.`folder_synckey` "
		qry += "(`user_uid`, `device_id`, `history_id`, `timestamp`) "
		qry += "VALUE(?, ?, ?, NOW())"
		result, err := tx.Exec(qry, r.userUID, r.deviceID, historyID)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		syncKey = uint64(id)
		return nil
	}

	if err := r.queryer.Query(f); err != nil {
		return 0, err
	}
	return syncKey, nil
}

func (r *folderSync) GetLastSyncKey(lock database.LockMode) (syncKey uint64, ok bool, err error) {
	f := func(tx *sql.Tx) error {
		qry := "SELECT `id` FROM `" + r.dbName + "`.`folder_synckey` "
		qry += "WHERE `user_uid` = ? AND `device_id` = ? "
		qry += "ORDER BY `id` DESC LIMIT 1 "
		qry += mysql.GetLockCmd(lock)
		if err := tx.QueryRow(qry, r.userUID, r.deviceID).Scan(&syncKey); err != nil {
			if err != sql.ErrNoRows {
				return err
			}
			ok = false
		} else {
			ok = true
		}

		return nil
	}
	if err := r.queryer.Query(f); err != nil {
		return 0, false, err
	}

	return syncKey, ok, nil
}

func (r *folderSync) ClearSyncKeys() error {
	f := func(tx *sql.Tx) error {
		qry := "DELETE FROM `" + r.dbName + "`.`folder_synckey` WHERE `user_uid` = ? AND `device_id` = ?"
		if _, err := tx.Exec(qry, r.userUID, r.deviceID); err != nil {
			return err
		}
		return nil
	}

	return r.queryer.Query(f)
}

func (r *folderSync) ClearVirtualFolders() error {
	f := func(tx *sql.Tx) error {
		qry := "DELETE FROM `" + r.dbName + "`.`virtual_folder` WHERE `user_uid` = ? AND `device_id` = ?"
		if _, err := tx.Exec(qry, r.userUID, r.deviceID); err != nil {
			return err
		}
		return nil
	}

	return r.queryer.Query(f)
}

func (r *folderSync) AddVirtualFolder(folder backend.Folder, lastHistoryID uint64) error {
	f := func(tx *sql.Tx) error {
		qry := "INSERT INTO `" + r.dbName + "`.`virtual_folder` "
		qry += "(`user_uid`, `device_id`, `folder_id`, `parent_folder_id`, `name`, `last_history_id`, `timestamp`) "
		qry += "VALUE(?, ?, ?, ?, ?, ?, NOW())"

		if _, err := tx.Exec(qry, r.userUID, r.deviceID, folder.ID, folder.ParentID, folder.Name, lastHistoryID); err != nil {
			return err
		}
		return nil
	}

	return r.queryer.Query(f)
}

func (r *folderSync) GetVirtualFolder(folderID uint64, lock database.LockMode) (folder activesync.VirtualFolder, err error) {
	f := func(tx *sql.Tx) error {
		qry := "SELECT `folder_id`, `parent_folder_id`, `name`, `last_history_id` "
		qry += "FROM `" + r.dbName + "`.`virtual_folder` "
		qry += "WHERE `user_uid` = ? AND `device_id` = ? AND `folder_id` = ? "
		qry += mysql.GetLockCmd(lock)
		if err := tx.QueryRow(qry, r.userUID, r.deviceID, folderID).Scan(&folder.ID, &folder.ParentID, &folder.Name, &folder.LastHistoryID); err != nil {
			return err
		}

		return nil
	}

	if err := r.queryer.Query(f); err != nil {
		return activesync.VirtualFolder{}, err
	}
	return folder, nil
}

func (r *folderSync) UpdateVirtualFolder(folder backend.Folder) error {
	f := func(tx *sql.Tx) error {
		qry := "UPDATE `" + r.dbName + "`.`virtual_folder` "
		qry += "SET `parent_folder_id` = ?, `name` = ? "
		qry += "WHERE `user_uid` = ? AND `device_id` = ? AND `folder_id` = ?"

		result, err := tx.Exec(qry, folder.ParentID, folder.Name, r.userUID, r.deviceID, folder.ID)
		if err != nil {
			return err
		}
		// NOTE: Assume that clientFoundRows is enabled.
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return database.ErrNotFound
		}

		return nil
	}

	if err := r.queryer.Query(f); err != nil {
		return err
	}

	return nil
}

func (r *folderSync) RemoveVirtualFolder(folderID uint64) error {
	f := func(tx *sql.Tx) error {
		qry := "DELETE FROM `" + r.dbName + "`.`virtual_folder` "
		qry += "WHERE `user_uid` = ? AND `device_id` = ? AND `folder_id` = ?"
		result, err := tx.Exec(qry, r.userUID, r.deviceID, folderID)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return database.ErrNotFound
		}

		return nil
	}

	if err := r.queryer.Query(f); err != nil {
		return err
	}

	return nil
}
