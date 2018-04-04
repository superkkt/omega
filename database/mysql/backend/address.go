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
	"net/mail"
	"strings"

	"github.com/superkkt/omega/backend"
)

func parseAddress(address string) backend.EmailAddress {
	v, err := mail.ParseAddress(address)
	if err != nil {
		// fallback
		return backend.EmailAddress{
			Name:    "",
			Address: address,
		}
	}

	return backend.EmailAddress{
		Name:    v.Name,
		Address: v.Address,
	}
}

func parseAddressList(address string) []backend.EmailAddress {
	arr := strings.Split(address, ",")

	addrs := make([]backend.EmailAddress, len(arr))
	for i, v := range arr {
		addrs[i] = parseAddress(v)
	}

	return addrs
}
