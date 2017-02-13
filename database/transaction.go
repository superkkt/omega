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

package database

import (
	"database/sql"
	"errors"
)

var (
	ErrNotFound   = errors.New("not found row")
	ErrDuplicated = errors.New("duplicated row")
)

type TransactionManager interface {
	NewTransaction() Transaction
}

// Transaction provides a database transaction. Transaction should return TransactionError
// when an error occurs.
type Transaction interface {
	Queryer
	// Begin starts this transactoin. Begin should start a new fresh transaction if it
	// is called on an already finished (committed or rollbacked) transaction.
	Begin() error
	Commit() error
	Rollback() error
	// Error returns the last TransactionError, if any, that was encountered during
	// query and commit processes.
	Error() TransactionError
}

type Queryer interface {
	Query(func(*sql.Tx) error) error
}

type TransactionError interface {
	error
	DeadlockError
	NotFoundError
	DuplicatedError
}

type DeadlockError interface {
	IsDeadlock() bool
}

type NotFoundError interface {
	IsNotFound() bool
}

type DuplicatedError interface {
	IsDuplicated() bool
}

type LockMode int

const (
	LockNone LockMode = iota
	LockRead
	LockWrite
)
