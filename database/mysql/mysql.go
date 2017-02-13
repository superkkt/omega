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

package mysql

import (
	"database/sql"
	"fmt"

	"github.com/Muzikatoshi/omega/database"

	"github.com/go-sql-driver/mysql"
)

const (
	maxIdleConn          = 8
	maxOpenConn          = 24
	mysqlDeadlockError   = 1213
	mysqlDuplicatedError = 1062
)

// XXX: Do not share variables in MySQL without exclusive locking. MySQL may be used by muliple goroutines simultaneously.
type MySQL struct {
	handle *sql.DB
}

func NewMySQL(host, username, password string, port uint16, clientFoundRows bool) (*MySQL, error) {
	dsn := fmt.Sprintf("%v:%v@tcp(%v:%v)/?timeout=5s&parseTime=true&loc=Local&clientFoundRows=%v", username, password, host, port, clientFoundRows)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening MySQL database: %v", err)
	}
	db.SetMaxOpenConns(maxOpenConn)
	db.SetMaxIdleConns(maxIdleConn)
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("connecting to a MySQL database server: %v", err)
	}

	return &MySQL{
		handle: db,
	}, nil
}

func isDeadlock(err error) bool {
	e, ok := err.(*mysql.MySQLError)
	if !ok {
		return false
	}

	return e.Number == mysqlDeadlockError
}

func isNotFound(err error) bool {
	return err == sql.ErrNoRows || err == database.ErrNotFound
}

func isDuplicated(err error) bool {
	if err == database.ErrDuplicated {
		return true
	}

	e, ok := err.(*mysql.MySQLError)
	if !ok {
		return false
	}
	return e.Number == mysqlDuplicatedError
}

func GetLockCmd(lock database.LockMode) string {
	// Return value should have extra spaces to avoid malformed query string.
	switch lock {
	case database.LockNone:
		return " "
	case database.LockRead:
		return " LOCK IN SHARE MODE "
	case database.LockWrite:
		return " FOR UPDATE "
	default:
		panic("unexpected lock mode")
	}
}
