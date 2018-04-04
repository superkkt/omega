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

package eas25

import (
	"encoding/xml"

	"github.com/superkkt/omega/database"
)

func (r *handler) handleGetHierarchy(tx database.Transaction) error {
	// GetHierarchy response is given in WBXML encoding.
	r.resp.SetWBXML(true)

	// NOTE: GetHierarchy does not have a XML body in the request.
	manager := r.param.BackendStorage.NewFolderManager(tx, r.credential)
	folders, err := manager.GetFolders(database.LockNone)
	if err != nil {
		return err
	}

	resp := &GetHierarchyResp{
		// The FolderHierarchy namespace is the primary namespace.
		NS: "FolderHierarchy:",
	}
	for _, v := range folders {
		resp.Folder = append(resp.Folder, Folder{ServerId: v.ID, ParentId: v.ParentID, DisplayName: v.Name, Type: getASFolderType(v)})
	}
	output, err := xml.Marshal(resp)
	if err != nil {
		return err
	}
	r.resp.Write(output)

	return nil
}

type GetHierarchyResp struct {
	// Folders is the top level element, not GetHierarchy.
	XMLName xml.Name `xml:"Folders"`
	NS      string   `xml:"xmlns,attr"`
	Folder  []Folder `xml:",omitempty"`
}
