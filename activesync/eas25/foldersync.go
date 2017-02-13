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

package eas25

import (
	"encoding/xml"
	"errors"
	"fmt"

	"github.com/Muzikatoshi/omega/activesync"
	"github.com/Muzikatoshi/omega/backend"
	"github.com/Muzikatoshi/omega/database"

	"github.com/superkkt/logger"
)

func (r *handler) handleFolderSync(tx database.Transaction) error {
	// FolderSync response is given in WBXML encoding.
	r.resp.SetWBXML(true)

	reqBody := struct {
		XMLName xml.Name `xml:"FolderSync"`
		SyncKey uint64
	}{}
	if err := activesync.ParseWBXMLRequest(r.req, &reqBody); err != nil {
		r.badRequest = true
		return fmt.Errorf("ParseWBXMLRequest: %v", err)
	}
	logger.Debug(fmt.Sprintf("FolderSync request: %+v", reqBody))

	fs := r.param.ASStorage.NewFolderSync(tx, r.credential.UserUID(), getDeviceID(r.req))
	manager := r.param.BackendStorage.NewFolderManager(tx, r.credential)

	var err error
	var response FolderSyncResp
	if reqBody.SyncKey == 0 {
		response, err = r.initialFolderSync(fs, manager)
	} else {
		response, err = r.folderSync(fs, manager, reqBody.SyncKey)
	}
	if err != nil {
		return fmt.Errorf("failed to sync folders: %v", err)
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

// initialFolderSync handles the initial FolderSync request whose SyncKey is 0.
func (r *handler) initialFolderSync(fs activesync.FolderSync, manager backend.FolderManager) (FolderSyncResp, error) {
	logger.Debug(fmt.Sprintf("Initial folder synchronizing: IP=%v, UserUID=%v, DeviceID=%v", r.req.RemoteAddr, fs.UserUID(), fs.DeviceID()))

	if err := fs.ClearSyncKeys(); err != nil {
		return FolderSyncResp{}, err
	}
	logger.Debug("Cleared the folder sync key table!")
	if err := fs.ClearVirtualFolders(); err != nil {
		return FolderSyncResp{}, err
	}
	logger.Debug("Cleared the virtual folder table!")

	// Use a read lock to preserve the last history entry until we create a new sync key based on it.
	lastHistory, err := manager.GetFolderHistories(0, 1, true, database.LockRead)
	if err != nil {
		return FolderSyncResp{}, err
	}
	logger.Debug(fmt.Sprintf("Last history ID = %v", lastHistory))

	var newSyncKey uint64
	if len(lastHistory) == 0 {
		newSyncKey, err = fs.NewSyncKey(0)
	} else {
		newSyncKey, err = fs.NewSyncKey(lastHistory[0].ID())
	}
	if err != nil {
		return FolderSyncResp{}, err
	}
	logger.Debug(fmt.Sprintf("New SyncKey = %v", newSyncKey))

	folders, err := manager.GetFolders(database.LockNone)
	if err != nil {
		return FolderSyncResp{}, err
	}
	add := make([]FolderOperation, 0)
	for _, v := range folders {
		// We can assume the last history ID related with this folder is
		// zero because this initial sync ensures there is no pending
		// histories to be sync.
		if err := fs.AddVirtualFolder(v, 0); err != nil {
			return FolderSyncResp{}, err
		}
		add = append(add, FolderSyncAdd{Folder: Folder{ServerId: v.ID, ParentId: v.ParentID, DisplayName: truncateFolderName(v.Name), Type: getASFolderType(v)}})
	}
	logger.Debug(fmt.Sprintf("Synced %v folders", len(add)))

	return FolderSyncResp{
		Status:  1,
		SyncKey: newSyncKey,
		Changes: &FolderSyncChange{
			Count:      len(add),
			Operations: add,
		},
	}, nil
}

type FolderSyncResp struct {
	XMLName xml.Name `xml:"FolderSync"`
	NS      string   `xml:"xmlns,attr"`
	Status  int
	SyncKey uint64            `xml:",omitempty"`
	Changes *FolderSyncChange `xml:",omitempty"`
}

type FolderSyncChange struct {
	Count      int
	Operations []FolderOperation
}

type FolderOperation interface{} // One of FolderSyncAdd, FolderSyncDelete, and FolderSyncUpdate

type FolderSyncAdd struct {
	XMLName xml.Name `xml:"Add"`
	Folder
}

type FolderSyncDelete struct {
	XMLName  xml.Name `xml:"Delete"`
	ServerId uint64
}

type FolderSyncUpdate struct {
	XMLName xml.Name `xml:"Update"`
	Folder
}

type Folder struct {
	ServerId    uint64
	ParentId    uint64
	DisplayName string
	Type        int
}

// Only support email folders
func getASFolderType(f backend.Folder) int {
	switch f.Type {
	case backend.EmailInbox:
		return 2
	case backend.EmailDraft:
		return 3
	case backend.EmailTrash:
		return 4
	case backend.EmailSent:
		return 5
	case backend.EmailOutbox:
		return 6
	case backend.EmailFolder:
		return 12
	default:
		return 1 // User-created folder (generic)
	}
}

var errUnsupportedFolderType = errors.New("unsupported folder type")

// Only support email folders
func getBackendFolderType(asType int) (backend.FolderType, error) {
	switch asType {
	case 1, 12:
		return backend.EmailFolder, nil
	case 2:
		return backend.EmailInbox, nil
	case 3:
		return backend.EmailDraft, nil
	case 4:
		return backend.EmailTrash, nil
	case 5:
		return backend.EmailSent, nil
	case 6:
		return backend.EmailOutbox, nil
	default:
		return 0, errUnsupportedFolderType
	}
}

func isNotFound(err error) bool {
	e, ok := err.(database.NotFoundError)
	if !ok {
		return false
	}

	return e.IsNotFound()
}

func isDuplicated(err error) bool {
	e, ok := err.(database.DuplicatedError)
	if !ok {
		return false
	}

	return e.IsDuplicated()
}

// folderSync handles subsequent FolderSync requests.
func (r *handler) folderSync(fs activesync.FolderSync, manager backend.FolderManager, syncKey uint64) (FolderSyncResp, error) {
	logger.Debug(fmt.Sprintf("Folder synchronizing: IP=%v, UserUID=%v, DeviceID=%v, SyncKey=%v", r.req.RemoteAddr, fs.UserUID(), fs.DeviceID(), syncKey))

	// Use a write lock to make sure we sequentially process concurrent requests that have same sync key.
	historyID, err := fs.LoadSyncKey(syncKey, database.LockWrite)
	if err != nil {
		if !isNotFound(err) {
			return FolderSyncResp{}, err
		}
		logger.Error(fmt.Sprintf("Client sent unknown folder sync key: IP=%v, UserUID=%v, DeviceID=%v, SyncKey=%v", r.req.RemoteAddr, fs.UserUID(), fs.DeviceID(), syncKey))
		// Ask folder full sync
		return FolderSyncResp{Status: 9}, nil
	}
	logger.Debug(fmt.Sprintf("History ID = %v", historyID))

	lastSyncKey, ok, err := fs.GetLastSyncKey(database.LockNone)
	if err != nil {
		return FolderSyncResp{}, err
	}
	if !ok {
		// It should exist because we succeed to load the syncKey the client sent.
		panic("lastSyncKey should exist")
	}
	// Does client send the previous syncKey that is already processed before?
	if lastSyncKey != syncKey {
		logger.Error(fmt.Sprintf("Client sent corrupted folder sync key: IP=%v, UserUID=%v, DeviceID=%v, lastSyncKey=%v, sentSyncKey=%v", r.req.RemoteAddr, fs.UserUID(), fs.DeviceID(), lastSyncKey, syncKey))
		// Send the last SyncKey we assigned.
		return FolderSyncResp{Status: 1, SyncKey: lastSyncKey}, nil
	}

	// Use a read lock to preserve the folder histories until we create virtual table entries.
	histories, err := manager.GetFolderHistories(historyID+1, maxQueryRows, false, database.LockRead)
	if err != nil {
		return FolderSyncResp{}, err
	}
	if len(histories) == 0 {
		logger.Debug(fmt.Sprintf("No folder changes! NewSyncKey=%v", syncKey))
		// No changes. Send same syncKey that the client sent.
		return FolderSyncResp{Status: 1, SyncKey: syncKey}, nil
	}
	if len(histories) == maxQueryRows {
		logger.Error(fmt.Sprintf("Client needs to update too many histories. So instead, we are asking the client re-fullsync again: IP=%v, UserUID=%v, DeviceID=%v, SyncKey=%v", r.req.RemoteAddr, fs.UserUID(), fs.DeviceID(), syncKey))
		// Ask folder full sync
		return FolderSyncResp{Status: 9}, nil
	}

	affected, err := applyHistories(fs, manager, histories)
	if err != nil {
		return FolderSyncResp{}, err
	}
	newSyncKey, err := fs.NewSyncKey(histories[len(histories)-1].ID())
	if err != nil {
		return FolderSyncResp{}, err
	}
	logger.Debug(fmt.Sprintf("New SyncKey = %v", newSyncKey))

	return FolderSyncResp{
		Status:  1,
		SyncKey: newSyncKey,
		Changes: &FolderSyncChange{
			Count:      len(affected),
			Operations: affected,
		},
	}, nil
}

// getLastFolderHistoryID returns the last history's ID related with folderID.
// It will return 0 if there is no hisotry about the folder.
func getLastFolderHistoryID(manager backend.FolderManager, folderID uint64) (uint64, error) {
	h, err := manager.GetLastFolderHistory(folderID, database.LockRead)
	if err != nil {
		if !isNotFound(err) {
			return 0, err
		}
		// Not found
		return 0, nil
	}

	return h.ID(), nil
}

// applyHistories applies histories to the virtual folder and returns affected histories
// except duplicated one, which are already applied on the virtual folder.
func applyHistories(fs activesync.FolderSync, manager backend.FolderManager, histories []backend.FolderHistory) ([]FolderOperation, error) {
	op := make([]FolderOperation, 0)

	for _, hist := range histories {
		folder, err := hist.Value()
		if err != nil {
			return nil, err
		}

		notFound := false
		// Use a write lock for updating the virtual folder.
		virt, err := fs.GetVirtualFolder(folder.ID, database.LockWrite)
		if err != nil {
			if !isNotFound(err) {
				return nil, err
			}
			notFound = true
		}

		switch hist.Operation() {
		case backend.FolderAdd:
			// Does the user still have this folder? Use a read lock to preserve this folder until we create a new virtual folder.
			latest, err := manager.GetFolderByID(folder.ID, database.LockRead)
			if err != nil {
				if !isNotFound(err) {
					return nil, err
				}
				// Not found in the backend database. Skip this add history so that
				// ignore all useless subsequent histories related to this one.
				logger.Debug(fmt.Sprintf("ADD: FolderID=%v, skip because it does not exist in the backend database", folder.ID))
				continue
			}
			if !notFound {
				logger.Debug(fmt.Sprintf("ADD: FolderID=%v, skip because it already exists in the virtual database", folder.ID))
				continue
			}
			lastChange, err := getLastFolderHistoryID(manager, folder.ID)
			if err != nil {
				return nil, err
			}
			// Use the latest folder data, instead of one from the history, to update
			// its flags to the latest values. This allows that useless subsequent
			// histories are automatically skipped by checking the virtual folder.
			if err := fs.AddVirtualFolder(latest, lastChange); err != nil {
				return nil, err
			}
			op = append(op, FolderSyncAdd{Folder: Folder{ServerId: latest.ID, ParentId: latest.ParentID, DisplayName: truncateFolderName(latest.Name), Type: getASFolderType(latest)}})
			logger.Debug(fmt.Sprintf("Added: FolderID=%v", folder.ID))
		case backend.FolderDelete:
			if notFound {
				logger.Debug(fmt.Sprintf("DELETE: FolderID=%v, skip because it does not exist in the virtual database", folder.ID))
				continue
			}
			if err := fs.RemoveVirtualFolder(folder.ID); err != nil {
				return nil, err
			}
			op = append(op, FolderSyncDelete{ServerId: folder.ID})
			logger.Debug(fmt.Sprintf("Deleted: FolderID=%v", folder.ID))
		case backend.FolderUpdate:
			if notFound || (folder.Name == virt.Name && folder.ParentID == virt.ParentID) || hist.ID() <= virt.LastHistoryID {
				logger.Debug(fmt.Sprintf("UPDATE: FolderID=%v, skip because it does not exist in the virtual database, has same values, or is already processed", folder.ID))
				continue
			}
			if err := fs.UpdateVirtualFolder(folder); err != nil {
				return nil, err
			}
			op = append(op, FolderSyncUpdate{Folder: Folder{ServerId: folder.ID, ParentId: folder.ParentID, DisplayName: truncateFolderName(folder.Name), Type: getASFolderType(folder)}})
			logger.Debug(fmt.Sprintf("Updated: Folder ID=%v, Name=%v, ParentID=%v", folder.ID, folder.Name, folder.ParentID))
		default:
			panic(fmt.Sprintf("Unexpected folder history's operation: %v", hist.Operation()))
		}
	}

	return op, nil
}

func truncateFolderName(name string) string {
	v := []rune(name)
	if len(v) <= 256 {
		return name
	}

	return string(v[0:256])
}
