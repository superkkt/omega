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

package mysql

import (
	"database/sql"
	"errors"
	"sync"

	"github.com/superkkt/omega/database"
)

func (r *MySQL) NewTransaction() database.Transaction {
	return newTransaction(r.handle)
}

type transaction struct {
	mu       sync.Mutex
	handle   *sql.DB
	tx       *sql.Tx
	finished bool
	lastErr  txError
}

func newTransaction(handle *sql.DB) database.Transaction {
	return &transaction{
		handle: handle,
	}
}

func (r *transaction) Begin() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.tx != nil && !r.finished {
		return newTxError(errors.New("begin on a valid transaction being used"))
	}

	return r.begin()
}

// The mutex should be locked by the caller.
func (r *transaction) begin() error {
	tx, err := r.handle.Begin()
	if err != nil {
		return newTxError(err)
	}
	r.tx = tx
	r.finished = false

	return nil
}

func (r *transaction) Query(f func(*sql.Tx) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.tx == nil {
		return newTxError(errors.New("invalid transaction"))
	}
	if r.finished {
		return newTxError(errors.New("query on an already finished transaction"))
	}

	if err := f(r.tx); err != nil {
		r.lastErr = newTxError(err)
		return r.lastErr
	}

	return nil
}

func (r *transaction) Commit() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.tx == nil {
		return newTxError(errors.New("invalid transaction"))
	}
	if r.finished {
		return newTxError(errors.New("commit on an already finished transaction"))
	}

	if err := r.tx.Commit(); err != nil {
		r.lastErr = newTxError(err)
		return r.lastErr
	}
	r.finished = true

	return nil
}

func (r *transaction) Rollback() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.tx == nil {
		return newTxError(errors.New("invalid transaction"))
	}
	if r.finished {
		return newTxError(errors.New("rollback on an already finished transaction"))
	}

	if err := r.tx.Rollback(); err != nil {
		return newTxError(err)
	}
	r.finished = true

	return nil
}

func (r *transaction) Error() database.TransactionError {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.lastErr
}

type txError struct {
	err        error
	deadlock   bool
	notFound   bool
	duplicated bool
}

func newTxError(err error) txError {
	return txError{
		err:        err,
		deadlock:   isDeadlock(err),
		notFound:   isNotFound(err),
		duplicated: isDuplicated(err),
	}
}

func (r txError) IsDeadlock() bool {
	return r.deadlock
}

func (r txError) IsNotFound() bool {
	return r.notFound
}

func (r txError) IsDuplicated() bool {
	return r.duplicated
}

func (r txError) Error() string {
	return r.err.Error()
}
