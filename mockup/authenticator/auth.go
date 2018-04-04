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

package authenticator

import "github.com/superkkt/omega/backend"

// TODO: remove this mockup and implement a real authenticator.
type MockAuth struct {
	Username string
	Password string
}

func (r *MockAuth) Auth(userID, password string) (backend.Credential, error) {
	if userID == r.Username && password == r.Password {
		return &mockCredential{auth: true, userUID: 1, userID: userID}, nil
	}

	return &mockCredential{userID: userID}, nil
}

type mockCredential struct {
	auth    bool
	userUID uint64
	userID  string
}

func (r *mockCredential) IsAuthorized() bool {
	return r.auth
}

func (r *mockCredential) UserID() string {
	return r.userID
}

func (r *mockCredential) UserUID() uint64 {
	return r.userUID
}
