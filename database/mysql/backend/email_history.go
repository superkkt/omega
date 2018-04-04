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
	"time"

	"github.com/superkkt/omega/backend"
)

type EmailHistory struct {
	id        uint64
	operation backend.EmailOperation
	value     backend.Email
	timestamp time.Time
}

func (e *EmailHistory) ID() uint64 {
	return e.id
}

func (e *EmailHistory) Operation() backend.EmailOperation {
	return e.operation
}

func (e *EmailHistory) Value() (*backend.Email, error) {
	return &e.value, nil
}

func (e *EmailHistory) Timestamp() time.Time {
	return e.timestamp
}
