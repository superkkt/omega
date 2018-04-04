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
	"fmt"

	"github.com/superkkt/omega/activesync"
	"github.com/superkkt/omega/backend"
	"github.com/superkkt/omega/database"

	"github.com/superkkt/logger"
)

type FolderUpdateReq struct {
	XMLName     xml.Name `xml:"FolderUpdate"`
	SyncKey     uint64
	ServerId    uint64
	ParentId    uint64
	DisplayName string
}

func (r *handler) handleFolderUpdate(tx database.Transaction) error {
	// FolderUpdate response is given in WBXML encoding.
	r.resp.SetWBXML(true)

	reqBody := new(FolderUpdateReq)
	if err := activesync.ParseWBXMLRequest(r.req, &reqBody); err != nil {
		r.badRequest = true
		return fmt.Errorf("ParseWBXMLRequest: %v", err)
	}
	logger.Debug(fmt.Sprintf("FolderUpdate request: %+v", reqBody))

	fs := r.param.ASStorage.NewFolderSync(tx, r.credential.UserUID(), getDeviceID(r.req))
	manager := r.param.BackendStorage.NewFolderManager(tx, r.credential)
	response, err := r.folderUpdate(fs, manager, reqBody)
	if err != nil {
		return fmt.Errorf("failed to update folder: %v", err)
	}
	// Default XML namespace
	response.NS = "FolderHierarchy:"

	output, err := xml.Marshal(response)
	if err != nil {
		return err
	}
	r.resp.Write(output)

	return nil
}

func (r *handler) folderUpdate(fs activesync.FolderSync, manager backend.FolderManager, req *FolderUpdateReq) (FolderUpdateResp, error) {
	// Empty or too long folder name?
	if len(req.DisplayName) == 0 || len([]rune(req.DisplayName)) > 256 {
		// Malformed request
		return FolderUpdateResp{Status: 10}, nil
	}

	// Use a read lock to make sure we can load this syncKey in following routines.
	lastSyncKey, ok, err := fs.GetLastSyncKey(database.LockRead)
	if err != nil {
		return FolderUpdateResp{}, err
	}

	if !ok || lastSyncKey != req.SyncKey {
		logger.Warning(fmt.Sprintf("Client sent corrupted folder sync key: IP=%v, UserUID=%v, DeviceID=%v, lastSyncKey=%v, sentSyncKey=%v", r.req.RemoteAddr, fs.UserUID(), fs.DeviceID(), lastSyncKey, req.SyncKey))
		// Client sync status has been corrupted. Send status 9 that asks the client to do full sync again.
		return FolderUpdateResp{Status: 9}, nil
	}

	// Use a write lock to update the folder.
	folder, err := manager.GetFolderByID(req.ServerId, database.LockWrite)
	if err != nil {
		if isNotFound(err) {
			// The folder does not exist.
			return FolderUpdateResp{Status: 4}, nil
		}
		return FolderUpdateResp{}, err
	}
	// Updating a special folder like INBOX?
	if folder.Type != backend.EmailFolder {
		return FolderUpdateResp{Status: 2}, nil
	}

	if err := manager.UpdateFolder(req.ServerId, req.ParentId, req.DisplayName); err != nil {
		switch {
		case isNotFound(err):
			// The parent folder does not exist.
			return FolderUpdateResp{Status: 5}, nil
		case isDuplicated(err):
			// The parent folder already contains a folder that has this name.
			return FolderUpdateResp{Status: 2}, nil
		default:
			return FolderUpdateResp{}, err
		}
	}

	folder.ParentID = req.ParentId
	folder.Name = req.DisplayName
	if err := fs.UpdateVirtualFolder(folder); err != nil {
		return FolderUpdateResp{}, err
	}

	historyID, err := fs.LoadSyncKey(req.SyncKey, database.LockNone)
	if err != nil {
		return FolderUpdateResp{}, err
	}

	// historyID should not be changed. This command is not a folder sync!!
	newSyncKey, err := fs.NewSyncKey(historyID)
	if err != nil {
		return FolderUpdateResp{}, err
	}
	logger.Debug(fmt.Sprintf("Folder updated: FolderID=%v, ParentID=%v, FolderName=%v, IP=%v, UserUID=%v, DeviceID=%v", req.ServerId, req.ParentId, req.DisplayName, r.req.RemoteAddr, fs.UserUID(), fs.DeviceID()))

	return FolderUpdateResp{Status: 1, SyncKey: newSyncKey}, nil
}

type FolderUpdateResp struct {
	XMLName xml.Name `xml:"FolderUpdate"`
	NS      string   `xml:"xmlns,attr"`
	Status  int
	SyncKey uint64 `xml:",omitempty"`
}
