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
	"fmt"
	"time"

	"github.com/Muzikatoshi/omega/activesync"
	"github.com/Muzikatoshi/omega/backend"
	"github.com/Muzikatoshi/omega/database"
	"github.com/Muzikatoshi/omega/database/mysql"

	"github.com/superkkt/logger"
)

type synchronize struct {
	queryer  database.Queryer
	userUID  uint64
	deviceID string
	folderID uint64
	dbName   string
}

func (r *synchronize) UserUID() uint64 {
	return r.userUID
}

func (r *synchronize) DeviceID() string {
	return r.deviceID
}

func (r *synchronize) FolderID() uint64 {
	return r.folderID
}

func (r *synchronize) ClearSyncKeys() error {
	f := func(tx *sql.Tx) error {
		qry := "DELETE FROM `" + r.dbName + "`.`email_synckey` "
		qry += "WHERE `user_uid` = ? AND `device_id` = ? AND `folder_id` = ?"
		if _, err := tx.Exec(qry, r.userUID, r.deviceID, r.folderID); err != nil {
			return err
		}
		return nil
	}

	return r.queryer.Query(f)
}

func (r *synchronize) ClearVirtualEmails() error {
	f := func(tx *sql.Tx) error {
		qry := "DELETE FROM `" + r.dbName + "`.`virtual_email` "
		qry += "WHERE `user_uid` = ? AND `device_id` = ? AND `folder_id` = ?"
		if _, err := tx.Exec(qry, r.userUID, r.deviceID, r.folderID); err != nil {
			return err
		}
		return nil
	}

	return r.queryer.Query(f)
}

func (r *synchronize) LoadSyncKey(syncKey uint64, lock database.LockMode) (historyID uint64, err error) {
	f := func(tx *sql.Tx) error {
		qry := "SELECT `history_id` "
		qry += "FROM `" + r.dbName + "`.`email_synckey` "
		qry += "WHERE `id` = ? AND `user_uid` = ? AND `device_id` = ? AND `folder_id` = ? "
		qry += mysql.GetLockCmd(lock)

		if err := tx.QueryRow(qry, syncKey, r.userUID, r.deviceID, r.folderID).Scan(&historyID); err != nil {
			return err
		}

		return nil
	}
	if err := r.queryer.Query(f); err != nil {
		return 0, err
	}

	return historyID, nil
}

func (r *synchronize) NewSyncKey(historyID uint64) (syncKey uint64, err error) {
	f := func(tx *sql.Tx) error {
		qry := "INSERT INTO `" + r.dbName + "`.`email_synckey` "
		qry += "(`user_uid`, `device_id`, `folder_id`, `history_id`, `timestamp`) "
		qry += "VALUE(?, ?, ?, ?, NOW())"

		result, err := tx.Exec(qry, r.userUID, r.deviceID, r.folderID, historyID)
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

func (r *synchronize) GetLastSyncKey(lock database.LockMode) (syncKey uint64, ok bool, err error) {
	f := func(tx *sql.Tx) error {
		qry := "SELECT `id` "
		qry += "FROM `" + r.dbName + "`.`email_synckey` "
		qry += "WHERE `user_uid` = ? AND `device_id` = ? AND `folder_id` = ? "
		qry += "ORDER BY `id` DESC LIMIT 1 "
		qry += mysql.GetLockCmd(lock)

		if err := tx.QueryRow(qry, r.userUID, r.deviceID, r.folderID).Scan(&syncKey); err != nil {
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

func (r *synchronize) GetOldestVirtualEmail(lock database.LockMode) (email activesync.VirtualEmail, ok bool, err error) {
	f := func(tx *sql.Tx) error {
		qry := "SELECT `email_id`, `seen`, `timestamp` "
		qry += "FROM `" + r.dbName + "`.`virtual_email` "
		qry += "WHERE `user_uid` = ? AND `device_id` = ? AND `folder_id` = ? "
		qry += "ORDER BY `email_id` ASC LIMIT 1 "
		qry += mysql.GetLockCmd(lock)

		if err := tx.QueryRow(qry, r.userUID, r.deviceID, r.folderID).Scan(&email.ID, &email.Seen, &email.Timestamp); err != nil {
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
		return activesync.VirtualEmail{}, false, err
	}
	return email, ok, nil
}

func (r *synchronize) AddVirtualEmail(email *backend.Email, lastHistoryID uint64) error {
	f := func(tx *sql.Tx) error {
		qry := "INSERT INTO `" + r.dbName + "`.`virtual_email` "
		qry += "(`user_uid`, `device_id`, `folder_id`, `email_id`, `seen`, `timestamp`, `last_history_id`) "
		qry += "VALUE(?, ?, ?, ?, ?, ?, ?)"

		if _, err := tx.Exec(qry, r.userUID, r.deviceID, r.folderID, email.ID, email.Seen, email.Date, lastHistoryID); err != nil {
			return err
		}

		return nil
	}

	return r.queryer.Query(f)
}

func (r *synchronize) GetOldVirtualEmails(threshold time.Time, limit uint, lock database.LockMode) (emails []activesync.VirtualEmail, err error) {
	logger.Debug(fmt.Sprintf("GetOldVirtualEmails: threshold = %v", threshold))

	f := func(tx *sql.Tx) error {
		qry := "SELECT `email_id`, `seen`, `timestamp` "
		qry += "FROM `" + r.dbName + "`.`virtual_email` "
		qry += "WHERE `user_uid` = ? AND `device_id` = ? AND `folder_id` = ? "
		qry += "ORDER BY `timestamp` ASC LIMIT ? "
		qry += mysql.GetLockCmd(lock)

		rows, err := tx.Query(qry, r.userUID, r.deviceID, r.folderID, limit)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			v := activesync.VirtualEmail{}
			if err := rows.Scan(&v.ID, &v.Seen, &v.Timestamp); err != nil {
				return err
			}
			logger.Debug(fmt.Sprintf("GetOldVirtualEmails: virtualEmail = %+v", v))
			if !v.Timestamp.Before(threshold) {
				logger.Debug(fmt.Sprintf("GetOldVirtualEmails: skip the virtualEmail = %+v", v))
				continue
			}
			emails = append(emails, v)
			logger.Debug(fmt.Sprintf("GetOldVirtualEmails: added virtualEmail to be soft-deleted = %+v", v))
		}
		if err := rows.Err(); err != nil {
			return err
		}
		return nil
	}
	if err := r.queryer.Query(f); err != nil {
		return nil, err
	}

	return emails, nil
}

func (r *synchronize) GetVirtualEmail(emailID uint64, lock database.LockMode) (email activesync.VirtualEmail, err error) {
	f := func(tx *sql.Tx) error {
		qry := "SELECT `seen`, `timestamp`, `last_history_id` "
		qry += "FROM `" + r.dbName + "`.`virtual_email` "
		qry += "WHERE `user_uid` = ? AND `device_id` = ? AND `folder_id` = ? AND `email_id` = ? "
		qry += mysql.GetLockCmd(lock)

		if err := tx.QueryRow(qry, r.userUID, r.deviceID, r.folderID, emailID).Scan(&email.Seen, &email.Timestamp, &email.LastHistoryID); err != nil {
			return err
		}
		email.ID = emailID

		return nil
	}

	if err := r.queryer.Query(f); err != nil {
		return activesync.VirtualEmail{}, err
	}
	return email, nil
}

func (r *synchronize) UpdateVirtualEmailSeen(emailID uint64, seen bool) error {
	f := func(tx *sql.Tx) error {
		qry := "UPDATE `" + r.dbName + "`.`virtual_email` "
		qry += "SET `seen` = ? "
		qry += "WHERE `user_uid` = ? AND `device_id` = ? AND `folder_id` = ? AND `email_id` = ?"

		result, err := tx.Exec(qry, seen, r.userUID, r.deviceID, r.folderID, emailID)
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

func (r *synchronize) UpdateVirtualEmailFolderID(emailID, folderID uint64) error {
	f := func(tx *sql.Tx) error {
		qry := "UPDATE `" + r.dbName + "`.`virtual_email` "
		qry += "SET `folder_id` = ? "
		qry += "WHERE `user_uid` = ? AND `device_id` = ? AND `folder_id` = ? AND `email_id` = ?"

		result, err := tx.Exec(qry, folderID, r.userUID, r.deviceID, r.folderID, emailID)
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

func (r *synchronize) RemoveVirtualEmail(emailID uint64) error {
	f := func(tx *sql.Tx) error {
		qry := "DELETE FROM `" + r.dbName + "`.`virtual_email` "
		qry += "WHERE `user_uid` = ? AND `device_id` = ? AND `folder_id` = ? AND `email_id` = ?"
		result, err := tx.Exec(qry, r.userUID, r.deviceID, r.folderID, emailID)
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
