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

package activesync

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/superkkt/omega/backend"
	"github.com/superkkt/omega/database"
)

var (
	factories factoryMap = factoryMap{}
)

type Factory interface {
	// Commands returns supported commands on this handler.
	Commands() []string
	New(Parameter) Handler
	// Version returns the version of this handler.
	Version() string
}

type Parameter struct {
	Authenticator  backend.Authenticator
	ASStorage      Storage
	BackendStorage backend.Storage
	Transaction    database.TransactionManager
	Mailer         Mailer
}

type Mailer interface {
	Send(from string, to []string, msg []byte) error
}

type Handler interface {
	Handle(c backend.Credential, w http.ResponseWriter, req *http.Request)
}

type factoryMap map[string]Factory

func (r factoryMap) Versions() string {
	// Convert map into slice
	s := make([]string, 0)
	for k, _ := range r {
		s = append(s, k)
	}
	sort.Sort(sortByVersion(s))

	return strings.Join(s, ",")
}

type sortByVersion []string

func (r sortByVersion) Len() int {
	return len(r)
}

func (r sortByVersion) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r sortByVersion) Less(i, j int) bool {
	v1, err := strconv.ParseFloat(r[i], 64)
	if err != nil {
		// Version should be numeric string that can be converted into float.
		panic(err)
	}
	v2, err := strconv.ParseFloat(r[j], 64)
	if err != nil {
		// Version should be numeric string that can be converted into float.
		panic(err)
	}

	return v1 < v2
}

func (r factoryMap) Commands() string {
	// Use map to eliminate duplicated commands
	m := make(map[string]struct{})
	for _, v := range r {
		cmds := v.Commands()
		for _, c := range cmds {
			m[c] = struct{}{}
		}
	}

	// Convert map into slice
	s := make([]string, 0)
	for k, _ := range m {
		s = append(s, k)
	}

	return strings.Join(s, ",")
}

func RegisterFactory(f Factory) {
	factories[f.Version()] = f
}
