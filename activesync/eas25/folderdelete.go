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

func (r *handler) handleFolderDelete(tx database.Transaction) error {
	// FolderDelete response is given in WBXML encoding.
	r.resp.SetWBXML(true)

	reqBody := struct {
		XMLName  xml.Name `xml:"FolderDelete"`
		SyncKey  uint64
		ServerId uint64
	}{}
	if err := activesync.ParseWBXMLRequest(r.req, &reqBody); err != nil {
		r.badRequest = true
		return fmt.Errorf("ParseWBXMLRequest: %v", err)
	}
	logger.Debug(fmt.Sprintf("FolderDelete request: %+v", reqBody))

	fs := r.param.ASStorage.NewFolderSync(tx, r.credential.UserUID(), getDeviceID(r.req))
	manager := r.param.BackendStorage.NewFolderManager(tx, r.credential)
	response, err := r.folderDelete(fs, manager, reqBody.SyncKey, reqBody.ServerId)
	if err != nil {
		return fmt.Errorf("failed to delete a folder: %v", err)
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

// folderDelete handles subsequent FolderDelete requests.
func (r *handler) folderDelete(fs activesync.FolderSync, manager backend.FolderManager, syncKey, folderID uint64) (FolderDeleteResp, error) {
	// Use a write lock on the folder to be deleted.
	folder, err := manager.GetFolderByID(folderID, database.LockWrite)
	if err != nil {
		if isNotFound(err) {
			// The folder does not exist.
			return FolderDeleteResp{Status: 4}, nil
		}
		return FolderDeleteResp{}, err
	}

	// Deleting a special folder like INBOX?
	if folder.Type != backend.EmailFolder {
		return FolderDeleteResp{Status: 3}, nil
	}

	// Use a read lock to make sure we can load this syncKey in following routines.
	lastSyncKey, ok, err := fs.GetLastSyncKey(database.LockRead)
	if err != nil {
		return FolderDeleteResp{}, err
	}
	if !ok || lastSyncKey != syncKey {
		logger.Warning(fmt.Sprintf("Client sent corrupted folder sync key: IP=%v, UserUID=%v, DeviceID=%v, lastSyncKey=%v, sentSyncKey=%v", r.req.RemoteAddr, fs.UserUID(), fs.DeviceID(), lastSyncKey, syncKey))
		// Client sync status has been corrupted. Send status 9 that asks the client to do full sync again.
		return FolderDeleteResp{Status: 9}, nil
	}

	if err := manager.DeleteFolder(folderID); err != nil {
		return FolderDeleteResp{}, err
	}
	if err := fs.RemoveVirtualFolder(folderID); err != nil {
		// Ignore not found error
		if !isNotFound(err) {
			return FolderDeleteResp{}, err
		}
	}

	historyID, err := fs.LoadSyncKey(syncKey, database.LockNone)
	if err != nil {
		return FolderDeleteResp{}, err
	}
	// historyID should not be changed. This command is not a folder sync!!
	newSyncKey, err := fs.NewSyncKey(historyID)
	if err != nil {
		return FolderDeleteResp{}, err
	}
	logger.Debug(fmt.Sprintf("Deleted a folder: FolderID=%v, FolderName=%v, IP=%v, UserUID=%v, DeviceID=%v", folderID, folder.Name, r.req.RemoteAddr, fs.UserUID(), fs.DeviceID()))

	return FolderDeleteResp{Status: 1, SyncKey: newSyncKey}, nil
}

type FolderDeleteResp struct {
	XMLName xml.Name `xml:"FolderDelete"`
	NS      string   `xml:"xmlns,attr"`
	Status  int
	SyncKey uint64 `xml:",omitempty"`
}
