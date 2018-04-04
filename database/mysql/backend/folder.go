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
	"database/sql"
	"errors"
	"fmt"

	"github.com/superkkt/omega/backend"
	"github.com/superkkt/omega/database"
	"github.com/superkkt/omega/database/mysql"
)

type FolderStorage struct {
	c       backend.Credential
	queryer database.Queryer
	dbName  string
}

func (r *FolderStorage) Credential() backend.Credential {
	return r.c
}

func (r *FolderStorage) GetFolders(lock database.LockMode) (folders []backend.Folder, err error) {
	f := func(tx *sql.Tx) error {
		qry := "SELECT `id`, `parent_id`, `name`, `type` "
		qry += fmt.Sprintf("FROM `%v`.`folder` ", r.dbName)
		qry += "WHERE user_id = ? AND available = true"
		qry += mysql.GetLockCmd(lock)

		rows, err := tx.Query(qry, r.c.UserUID())
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var t string
			v := backend.Folder{}
			if err := rows.Scan(&v.ID, &v.ParentID, &v.Name, &t); err != nil {
				return err
			}
			v.Type = ConvToBackendFolderType(t)
			folders = append(folders, v)
		}
		return nil
	}

	if err := r.queryer.Query(f); err != nil {
		return nil, err
	}
	return folders, nil
}

func (r *FolderStorage) GetFolderByID(folderID uint64, lock database.LockMode) (v backend.Folder, err error) {
	f := func(tx *sql.Tx) error {
		qry := "SELECT `id`, `parent_id`, `name`, `type` "
		qry += fmt.Sprintf("FROM `%v`.`folder` ", r.dbName)
		qry += "WHERE id = ? and user_id = ? and available = true"
		qry += mysql.GetLockCmd(lock)

		var t string
		if err := tx.QueryRow(qry, folderID, r.c.UserUID()).Scan(&v.ID, &v.ParentID, &v.Name, &t); err != nil {
			return err
		}
		v.Type = ConvToBackendFolderType(t)

		return nil
	}

	if err := r.queryer.Query(f); err != nil {
		return backend.Folder{}, err
	}
	return v, nil
}

func (r *FolderStorage) GetFolderByName(parentID uint64, name string, recursive bool, lock database.LockMode) (folders []backend.Folder, err error) {
	if recursive {
		return nil, errors.New("A true value of recursive is not supported.")
	}
	f := func(tx *sql.Tx) error {
		qry := "SELECT `id`, `parent_id`, `name`, `type` "
		qry += fmt.Sprintf("FROM `%v`.`folder` ", r.dbName)
		qry += "WHERE user_id = ? AND parent_id = ? AND name = ? AND available = true"
		qry += mysql.GetLockCmd(lock)

		rows, err := tx.Query(qry, r.c.UserUID(), parentID, name)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var t string
			v := backend.Folder{}
			if err := rows.Scan(&v.ID, &v.ParentID, &v.Name, &t); err != nil {
				return err
			}
			v.Type = ConvToBackendFolderType(t)
			folders = append(folders, v)
		}
		return nil
	}

	if err := r.queryer.Query(f); err != nil {
		return nil, err
	}
	return folders, nil
}

func (r *FolderStorage) GetFolderByType(t backend.FolderType, lock database.LockMode) (folders []backend.Folder, err error) {
	f := func(tx *sql.Tx) error {
		qry := "SELECT `id`, `parent_id`, `name`, `type` "
		qry += "FROM `" + r.dbName + "`.`folder` "
		qry += "WHERE user_id = ? AND `type` = ? AND available = true"
		qry += mysql.GetLockCmd(lock)

		rows, err := tx.Query(qry, r.c.UserUID(), ConvToFolderTypeString(t))
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var t string
			v := backend.Folder{}
			if err := rows.Scan(&v.ID, &v.ParentID, &v.Name, &t); err != nil {
				return err
			}
			v.Type = ConvToBackendFolderType(t)
			folders = append(folders, v)
		}
		return nil
	}

	if err := r.queryer.Query(f); err != nil {
		return nil, err
	}
	return folders, nil
}

func (r *FolderStorage) GetFolderByPath(path []string, lock database.LockMode) (folder backend.Folder, err error) {
	return backend.Folder{}, errors.New("unimplemented function")
}

//AddFolder should add a new folder history.
func (r *FolderStorage) AddFolder(parentID uint64, name string, t backend.FolderType) (folderID uint64, err error) {
	f := func(tx *sql.Tx) error {
		var qry string
		// If parent folder is not root.
		if parentID != 0 {
			// Check whether there is an existing parent folder, and then return database.ErrNotFound if it doesn't exist.
			qry = fmt.Sprintf("SELECT `id` FROM `%v`.`folder` ", r.dbName)
			qry += "WHERE id = ? AND user_id = ? AND available = TRUE "
			qry += "LOCK IN SHARE MODE"
			var id uint64
			if err := tx.QueryRow(qry, parentID, r.c.UserUID()).Scan(&id); err != nil {
				return err
			}
		}

		// Check whether there is an existing folder that has same name, and then return database.ErrDuplicated if it already exists.
		qry = fmt.Sprintf("SELECT COUNT(*) FROM `%v`.`folder` ", r.dbName)
		qry += "WHERE user_id = ? AND parent_id = ? AND name = ? AND available = TRUE "
		qry += "FOR UPDATE"
		var count uint64
		if err := tx.QueryRow(qry, r.c.UserUID(), parentID, name).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return database.ErrDuplicated
		}

		qry = fmt.Sprintf("INSERT INTO `%v`.`folder`(user_id, parent_id, name, `type`) ", r.dbName)
		qry += "VALUES(?, ?, ?, ?)"
		result, err := tx.Exec(qry, r.c.UserUID(), parentID, name, ConvToFolderTypeString(t))
		if err != nil {
			return err
		}
		fid, err := result.LastInsertId()
		if err != nil {
			return err
		}
		folderID = uint64(fid)

		qry = fmt.Sprintf("INSERT INTO `%v`.`folder_history`(user_id, folder_id, operation, parent_id, name) ", r.dbName)
		qry += "VALUES(?, ?, 'ADD', ?, ?)"
		if _, err := tx.Exec(qry, r.c.UserUID(), folderID, parentID, name); err != nil {
			return err
		}
		return nil
	}
	if err := r.queryer.Query(f); err != nil {
		return 0, err
	}
	return folderID, nil
}

// DeleteFolder should add a new folder history.
func (r *FolderStorage) DeleteFolder(folderID uint64) error {
	f := func(tx *sql.Tx) error {
		var parentID uint64
		var name string
		qry := fmt.Sprintf("SELECT `parent_id`, `name` FROM `%v`.`folder` ", r.dbName)
		qry += "WHERE id = ? and user_id = ? AND available = TRUE FOR UPDATE"
		if err := tx.QueryRow(qry, folderID, r.c.UserUID()).Scan(&parentID, &name); err != nil {
			return err
		}
		qry = fmt.Sprintf("UPDATE `%v`.`folder` SET available = FALSE WHERE id = ?", r.dbName)
		if _, err := tx.Exec(qry, folderID); err != nil {
			return err
		}
		qry = fmt.Sprintf("INSERT INTO `%v`.`folder_history`(user_id, folder_id, operation, parent_id, name) ", r.dbName)
		qry += "VALUES(?, ?, 'DEL', ?, ?)"
		if _, err := tx.Exec(qry, r.c.UserUID(), folderID, parentID, name); err != nil {
			return err
		}

		return nil
	}
	if err := r.queryer.Query(f); err != nil {
		return err
	}
	return nil
}

// UpdateFolder should add a new folder history.
func (r *FolderStorage) UpdateFolder(folderID, newParentID uint64, newName string) error {
	f := func(tx *sql.Tx) error {
		var qry string
		var id uint64
		// If parent folder is not root.
		if newParentID != 0 {
			// Check whether there is an existing parent folder, and then return database.ErrNotFound if it doesn't exist.
			qry = fmt.Sprintf("SELECT COUNT(*) FROM `%v`.`folder` ", r.dbName)
			qry += "WHERE user_id = ? AND parent_id = ? AND available = TRUE "
			qry += "LOCK IN SHARE MODE"
			if err := tx.QueryRow(qry, r.c.UserUID(), newParentID).Scan(&id); err != nil {
				return err
			}
		}

		// Check whether there is an existing folder whose name is newName under the new parent folder,
		// then return database.ErrDuplicated if it already exists.
		qry = fmt.Sprintf("SELECT COUNT(*) FROM `%v`.`folder` ", r.dbName)
		qry += "WHERE user_id = ? AND parent_id = ? AND name = ? AND available = TRUE "
		qry += "FOR UPDATE"
		var count uint64
		if err := tx.QueryRow(qry, r.c.UserUID(), newParentID, newName).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return database.ErrDuplicated
		}

		// Check whether there is an existing folder.
		qry = fmt.Sprintf("SELECT `id` FROM `%v`.`folder` ", r.dbName)
		qry += "WHERE id = ? AND user_id = ? AND available = TRUE "
		qry += "FOR UPDATE"
		if err := tx.QueryRow(qry, folderID, r.c.UserUID()).Scan(&id); err != nil {
			return err
		}

		qry = fmt.Sprintf("UPDATE `%v`.`folder` SET parent_id = ?, name = ? WHERE id = ?", r.dbName)
		result, err := tx.Exec(qry, newParentID, newName, folderID)
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

		qry = fmt.Sprintf("INSERT INTO `%v`.`folder_history`(user_id, folder_id, operation, parent_id, name) ", r.dbName)
		qry += "VALUES(?, ?, 'UPDATE', ?, ?)"
		if _, err := tx.Exec(qry, r.c.UserUID(), folderID, newParentID, newName); err != nil {
			return err
		}

		return nil
	}
	if err := r.queryer.Query(f); err != nil {
		return err
	}
	return nil
}

func (r *FolderStorage) GetLastFolderHistory(folderID uint64, lock database.LockMode) (history backend.FolderHistory, err error) {
	f := func(tx *sql.Tx) error {
		qry := "SELECT A.`id`, A.`operation`, A.`parent_id`, A.`name`, A.`timestamp`, B.`id`, B.`type` "
		qry += "FROM `" + r.dbName + "`.`folder_history` A INNER JOIN `" + r.dbName + "`.`folder` B "
		qry += "ON A.folder_id = B.id "
		qry += "WHERE B.`id` = ? AND A.user_id = ? "
		args := make([]interface{}, 0)
		args = append(args, folderID, r.c.UserUID())
		qry += "ORDER BY A.`id` DESC LIMIT 1"
		qry += mysql.GetLockCmd(lock)

		v := FolderHistory{}
		var foe, fte string
		var parentID sql.NullInt64
		if err := tx.QueryRow(qry, args...).Scan(&v.id, &foe, &parentID,
			&v.value.Name, &v.timestamp, &v.value.ID, &fte); err != nil {
			return err
		}
		v.operation = ConvToBackendFolderOperation(foe)
		v.value.Type = ConvToBackendFolderType(fte)
		if parentID.Valid {
			v.value.ParentID = uint64(parentID.Int64)
		}
		history = &v

		return nil
	}

	if err := r.queryer.Query(f); err != nil {
		return nil, err
	}
	return history, nil
}

// offset is a folder history ID as a starting position. desc means descending order if it is true,
// otherwise ascending order. Zero offset means the last item if desc is true. Zero limit means infinite.
func (r *FolderStorage) GetFolderHistories(offset, limit uint64, desc bool, lock database.LockMode) (histories []backend.FolderHistory, err error) {
	f := func(tx *sql.Tx) error {
		qry := "SELECT A.`id`, A.`operation`, A.`parent_id`, A.`name`, A.`timestamp`, B.`id`, B.`type` "
		qry += "FROM `" + r.dbName + "`.`folder_history` A INNER JOIN `" + r.dbName + "`.`folder` B "
		qry += "ON A.folder_id = B.id "
		qry += "WHERE A.user_id = ? "
		args := make([]interface{}, 0)
		args = append(args, r.c.UserUID())
		if offset != 0 {
			if desc {
				qry += "AND A.`id` <= ? "
			} else {
				qry += "AND A.`id` >= ? "
			}
			args = append(args, offset)
		}
		if desc {
			qry += "ORDER BY A.`id` DESC "
		} else {
			qry += "ORDER BY A.`id` ASC "
		}
		if limit != 0 {
			qry += "LIMIT ?"
			args = append(args, limit)
		}
		qry += mysql.GetLockCmd(lock)
		rows, err := tx.Query(qry, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			v := FolderHistory{}
			var foe, fte string
			var parentID sql.NullInt64
			if err := rows.Scan(&v.id, &foe, &parentID,
				&v.value.Name, &v.timestamp, &v.value.ID, &fte); err != nil {
				return err
			}
			v.operation = ConvToBackendFolderOperation(foe)
			v.value.Type = ConvToBackendFolderType(fte)
			if parentID.Valid {
				v.value.ParentID = uint64(parentID.Int64)
			}
			histories = append(histories, &v)
		}
		return nil
	}

	if err := r.queryer.Query(f); err != nil {
		return nil, err
	}
	return histories, nil
}

func (r *FolderStorage) DeleteFolderHistory(historyID uint64) error {
	f := func(tx *sql.Tx) error {
		qry := fmt.Sprintf("DELETE FROM `%v`.`folder_history` WHERE id = ? and user_id = ?", r.dbName)
		result, err := tx.Exec(qry, historyID, r.c.UserUID())
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
