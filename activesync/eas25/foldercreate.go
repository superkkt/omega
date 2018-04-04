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
	"net/http"

	"github.com/superkkt/omega/activesync"
	"github.com/superkkt/omega/backend"
	"github.com/superkkt/omega/database"

	"github.com/superkkt/logger"
)

type FolderCreateReq struct {
	XMLName     xml.Name `xml:"FolderCreate"`
	SyncKey     uint64
	ParentId    uint64
	DisplayName string
	Type        int
}

func (r *FolderCreateReq) FolderType() (backend.FolderType, error) {
	return getBackendFolderType(r.Type)
}

func (r *handler) handleFolderCreate(tx database.Transaction) error {
	// FolderCreate response is given in WBXML encoding.
	r.resp.SetWBXML(true)

	reqBody := new(FolderCreateReq)
	if err := activesync.ParseWBXMLRequest(r.req, reqBody); err != nil {
		r.badRequest = true
		return fmt.Errorf("ParseWBXMLRequest: %v", err)
	}
	logger.Debug(fmt.Sprintf("FolderCreate request: %+v", reqBody))

	_, err := reqBody.FolderType()
	if err != nil {
		// Only support email folders.
		if err == errUnsupportedFolderType {
			r.resp.WriteHeader(http.StatusNotImplemented)
			return nil
		}
		return fmt.Errorf("failed to convert folder type: %v", err)
	}

	fs := r.param.ASStorage.NewFolderSync(tx, r.credential.UserUID(), getDeviceID(r.req))
	manager := r.param.BackendStorage.NewFolderManager(tx, r.credential)
	response, err := r.folderCreate(fs, manager, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create folder: %v", err)
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

func (r *handler) folderCreate(fs activesync.FolderSync, manager backend.FolderManager, req *FolderCreateReq) (FolderCreateResp, error) {
	ft, err := req.FolderType()
	if err != nil {
		panic(fmt.Sprintf("unexpected error: %v", err))
	}
	// Creating a special folder like INBOX?
	if ft != backend.EmailFolder {
		// Malformed request
		return FolderCreateResp{Status: 10}, nil
	}

	// Empty or too long folder name?
	if len(req.DisplayName) == 0 || len([]rune(req.DisplayName)) > 256 {
		// Malformed request
		return FolderCreateResp{Status: 10}, nil
	}

	// Use a read lock to make sure we can load this syncKey in following routines.
	lastSyncKey, ok, err := fs.GetLastSyncKey(database.LockRead)
	if err != nil {
		return FolderCreateResp{}, err
	}
	if !ok || lastSyncKey != req.SyncKey {
		logger.Error(fmt.Sprintf("Client sent corrupted folder sync key: IP=%v, UserUID=%v, DeviceID=%v, lastSyncKey=%v, sentSyncKey=%v", r.req.RemoteAddr, fs.UserUID(), fs.DeviceID(), lastSyncKey, req.SyncKey))
		// Client sync status has been corrupted. Send status 9 that asks the client to do full sync again.
		return FolderCreateResp{Status: 9}, nil
	}

	folderID, err := manager.AddFolder(req.ParentId, req.DisplayName, ft)
	if err != nil {
		switch {
		case isNotFound(err):
			// The parent folder does not exist.
			return FolderCreateResp{Status: 5}, nil
		case isDuplicated(err):
			// The parent folder already contains a folder that has this name.
			return FolderCreateResp{Status: 2}, nil
		default:
			return FolderCreateResp{}, err
		}
	}
	// We can assume the last history ID related with this folder is zero
	// because the folder has been created just right now.
	if err := fs.AddVirtualFolder(backend.Folder{
		ID:       folderID,
		Name:     req.DisplayName,
		ParentID: req.ParentId,
		Type:     ft,
	}, 0); err != nil {
		// Ignore the duplicated error
		if !isDuplicated(err) {
			return FolderCreateResp{}, err
		}
	}

	historyID, err := fs.LoadSyncKey(req.SyncKey, database.LockNone)
	if err != nil {
		return FolderCreateResp{}, err
	}
	// historyID should not be changed. This command is not a folder sync!!
	newSyncKey, err := fs.NewSyncKey(historyID)
	if err != nil {
		return FolderCreateResp{}, err
	}
	logger.Debug(fmt.Sprintf("New folder is created: FolderID=%v, FolderName=%v, IP=%v, UserUID=%v, DeviceID=%v", folderID, req.DisplayName, r.req.RemoteAddr, fs.UserUID(), fs.DeviceID()))

	return FolderCreateResp{Status: 1, SyncKey: newSyncKey, ServerId: folderID}, nil
}

type FolderCreateResp struct {
	XMLName  xml.Name `xml:"FolderCreate"`
	NS       string   `xml:"xmlns,attr"`
	Status   int
	SyncKey  uint64 `xml:",omitempty"`
	ServerId uint64 `xml:",omitempty"`
}
