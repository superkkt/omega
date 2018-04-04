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
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/superkkt/omega/activesync"
	"github.com/superkkt/omega/backend"
	"github.com/superkkt/omega/database"

	"github.com/superkkt/logger"
)

const (
	maxSyncWindowSize = 100
)

type SyncReq struct {
	XMLName     xml.Name `xml:"Sync"`
	Collections struct {
		Collection []SyncCollection
	}
	WindowSize int // Global window size
}

func (r *SyncReq) NumCollections() int {
	return len(r.Collections.Collection)
}

func (r *SyncReq) HasClientChanges() bool {
	var count int

	for _, v := range r.Collections.Collection {
		count += len(v.Commands.Values)
	}

	return count > 0
}

type syncResp struct {
	syncKey      uint64
	collectionID uint64
	status       int
	moreAvail    bool
	commands     string
	responses    string
}

func (r *syncResp) encode() string {
	output := `<Sync xmlns="AirSync:" xmlns:email="Email:"><Collections><Collection><Class>Email</Class>`
	output += fmt.Sprintf(`<SyncKey>%v</SyncKey><CollectionId>%v</CollectionId><Status>%v</Status>`, r.syncKey, r.collectionID, r.status)
	if r.moreAvail {
		output += "<MoreAvailable/>"
	}
	if len(r.commands) > 0 {
		output += fmt.Sprintf(`<Commands>%v</Commands>`, r.commands)
	}
	if len(r.responses) > 0 {
		output += fmt.Sprintf(`<Responses>%v</Responses>`, r.responses)
	}
	output += "</Collection></Collections></Sync>"

	return output
}

type SyncCollection struct {
	Class          string
	SyncKey        uint64
	CollectionId   uint64
	DeletesAsMoves *string // boolean value, but we use string due to the self-closed tag.
	GetChanges     *string // boolean value, but we use string due to the self-closed tag.
	WindowSize     int     // Collection local window size
	Options        SyncOptions
	Commands       struct {
		// Some fields may be empty depending on its command type.
		Values []ClientChange `xml:",any"`
	}
}

func (r *SyncCollection) HasDeletesAsMoves() bool {
	// Only false if it is explicitly specified by the client
	if r.DeletesAsMoves != nil && *r.DeletesAsMoves == "0" {
		return false
	}

	return true
}

func (r *SyncCollection) HasGetChanges() bool {
	return r.GetChanges != nil
}

type ClientChange struct {
	// XMLName will have Add, Delete, Change, and Fetch in its Local name.
	XMLName         xml.Name
	ClientId        string
	ServerId        string
	ApplicationData Email
	Class           string
}

type SyncOptions struct {
	// If the FilterType element is not present, the default value of 0 is used.
	FilterType string
	// No explanation in the specification if the MIMETruncation is not present.
	MIMETruncation string
	// If the MIMESupport element is not present, the default value of 0 is used.
	MIMESupport string
	// If the Truncation element is not present, the value of 9 is used.
	Truncation string
}

// TODO: Use XML marshaler.
func (r *handler) handleSync(tx database.Transaction) error {
	// Sync response is given in WBXML encoding.
	r.resp.SetWBXML(true)

	reqBody := new(SyncReq)
	if err := activesync.ParseWBXMLRequest(r.req, reqBody); err != nil {
		r.badRequest = true
		return fmt.Errorf("ParseWBXMLRequest: %v", err)
	}
	logger.Debug(fmt.Sprintf("Sync request: %+v", reqBody))

	// Check folder existence. Use a read lock to preserve the folder until we finish this sync.
	fm := r.param.BackendStorage.NewFolderManager(tx, r.credential)
	folder, err := fm.GetFolderByID(reqBody.Collections.Collection[0].CollectionId, database.LockRead)
	if err != nil {
		if isNotFound(err) {
			// The folder hierarchy has changed. The client should perform FolderSync first.
			r.resp.Write([]byte(`<Sync xmlns="AirSync:"><Status>12</Status></Sync>`))
			logger.Warning(fmt.Sprintf("Sync request for an unknown folder: FolderID=%v", reqBody.Collections.Collection[0].CollectionId))
			return nil
		}
		return fmt.Errorf("failed to query a folder: %v", err)
	}

	// We only support a single collection.
	if reqBody.NumCollections() != 1 {
		var status int
		if reqBody.NumCollections() == 0 {
			// An empty or partial Sync command request is received and the cached set of notifyable collections is missing.
			status = 13
		} else {
			// Too many collections are included in the Sync request.
			status = 15
		}
		r.resp.Write([]byte(fmt.Sprintf(`<Sync xmlns="AirSync:"><Status>%v</Status></Sync>`, status)))
		logger.Warning(fmt.Sprintf("Sync request with an empty or too many collections: FolderID=%v, # of collections=%v", reqBody.Collections.Collection[0].CollectionId, reqBody.NumCollections()))
		return nil
	}

	// Assume the number of collections is always 1.
	collection := reqBody.Collections.Collection[0]
	sync := r.param.ASStorage.NewSync(tx, r.credential.UserUID(), getDeviceID(r.req), collection.CollectionId)
	em := r.param.BackendStorage.NewEmailManager(tx, r.credential, collection.CollectionId)
	resp := &syncResp{collectionID: collection.CollectionId}

	// NOTE: Make sure that there is no duplicated responses with the start and end one in following routines.
	if reqBody.HasClientChanges() {
		logger.Debug(fmt.Sprintf("Client sent client-side changes: IP=%v, UserUID=%v, DeviceID=%v, Request=%+v", r.req.RemoteAddr, r.credential.UserUID(), getDeviceID(r.req), reqBody))
		if err := r.applyClientChanges(sync, fm, em, collection, resp, folder); err != nil {
			return err
		}
	}

	if collection.SyncKey == 0 {
		err = r.initialSync(sync, em, collection, resp)
	} else {
		err = r.sync(sync, em, collection, reqBody, resp)
	}
	if err != nil {
		return fmt.Errorf("failed to sync: %v", err)
	}
	r.resp.Write([]byte(resp.encode()))

	return nil
}

func splitEmailID(serverID string) (uint64, error) {
	t := strings.Split(serverID, ":")
	if len(t) != 2 || len(t[1]) == 0 {
		return 0, fmt.Errorf("invalid serverID value: %v", serverID)
	}

	return strconv.ParseUint(t[1], 10, 64)
}

func (r *handler) applyClientChanges(sync activesync.Sync, fm backend.FolderManager, em backend.EmailManager, collection SyncCollection, resp *syncResp, folder backend.Folder) error {
	var output bytes.Buffer
	for _, v := range collection.Commands.Values {
		var o string
		var err error
		syncer := &clientChangeSyncer{
			sync:          sync,
			folderManager: fm,
			emailManager:  em,
			collection:    collection,
			folder:        folder,
			change:        v,
		}

		switch v.XMLName.Local {
		case "Add":
			o, err = syncer.syncAdd()
		case "Delete":
			o, err = syncer.syncDelete()
		case "Change":
			o, err = syncer.syncChange()
		case "Fetch":
			o, err = syncer.syncFetch()
		default:
			// Ignore unknown commands
			logger.Error(fmt.Sprintf("applyClientChanges: unknown command: %v", v.XMLName.Local))
			continue
		}
		if err != nil {
			if err == errBadRequest {
				r.badRequest = true
			}
			return err
		}
		if len(o) == 0 {
			continue
		}

		output.WriteString(o)
	}
	resp.responses = output.String()

	return nil
}

type clientChangeSyncer struct {
	sync          activesync.Sync
	folderManager backend.FolderManager
	emailManager  backend.EmailManager
	collection    SyncCollection
	folder        backend.Folder
	change        ClientChange
}

func (r *clientChangeSyncer) syncAdd() (output string, err error) {
	if r.change.Class != "Email" || len(r.change.ApplicationData.MIMEData) == 0 {
		// Protocol error. We only support an email folder.
		output = fmt.Sprintf("<Add><ClientId>%v</ClientId><Status>4</Status></Add>", r.change.ClientId)
		return output, nil
	}
	email, err := r.emailManager.AddEmail([]byte(r.change.ApplicationData.MIMEData))
	if err != nil {
		return "", err
	}
	// We can assume the last history ID related with this email is zero because the email has been added just right now.
	if err := r.sync.AddVirtualEmail(email, 0); err != nil {
		return "", err
	}
	output = fmt.Sprintf("<Add><ClientId>%v</ClientId><ServerId>%v:%v</ServerId><Class>Email</Class><Status>1</Status></Add>", r.change.ClientId, r.collection.CollectionId, email.ID)
	logger.Debug(fmt.Sprintf("Client-side ADD: ClientId=%v, ServerId=%v", r.change.ClientId, email.ID))

	return output, nil
}

var errBadRequest = errors.New("client sent a bad request")

func (r *clientChangeSyncer) syncDelete() (output string, err error) {
	emailID, err := splitEmailID(r.change.ServerId)
	if err != nil {
		return "", errBadRequest
	}

	if r.collection.HasDeletesAsMoves() && r.folder.Type != backend.EmailTrash {
		trash, err := r.folderManager.GetFolderByType(backend.EmailTrash, database.LockRead)
		if err != nil {
			return "", fmt.Errorf("failed to open a trash folder: %v", err)
		}
		if len(trash) == 0 {
			return "", errors.New("failed to open a trash folder: not found")
		}

		_, err = r.emailManager.MoveEmail(emailID, trash[0].ID)
		if err != nil {
			if !isNotFound(err) {
				return "", err
			}
			// Object not found.
			return fmt.Sprintf("<Delete><ServerId>%v</ServerId><Status>8</Status></Delete>", r.change.ServerId), nil
		}
		if err := r.sync.RemoveVirtualEmail(emailID); err != nil {
			// Ignore not found error
			if !isNotFound(err) {
				return "", err
			}
		}
	} else {
		if err := r.sync.RemoveVirtualEmail(emailID); err != nil {
			// Ignore not found error
			if !isNotFound(err) {
				return "", err
			}
		}
		if err := r.emailManager.DeleteEmail(emailID); err != nil {
			// Ignore not found error
			if !isNotFound(err) {
				return "", err
			}
		}
	}
	output = fmt.Sprintf("<Delete><ServerId>%v</ServerId><Status>1</Status></Delete>", r.change.ServerId)
	logger.Debug(fmt.Sprintf("Client-side DELETE: ServerId=%v (DeleteAsMoves: %v)", r.change.ServerId, r.collection.HasDeletesAsMoves()))

	return output, nil
}

func (r *clientChangeSyncer) syncChange() (output string, err error) {
	emailID, err := splitEmailID(r.change.ServerId)
	if err != nil {
		return "", errBadRequest
	}
	// We only support to change email seen value.
	if r.change.ApplicationData.Read != "" {
		seen := false
		if r.change.ApplicationData.Read == "1" {
			seen = true
		}
		if err := r.emailManager.UpdateEmail(emailID, seen); err != nil {
			if !isNotFound(err) {
				return "", err
			}
			// Object not found.
			return fmt.Sprintf("<Change><ServerId>%v</ServerId><Status>8</Status></Change>", r.change.ServerId), nil
		}
		if err := r.sync.UpdateVirtualEmailSeen(emailID, seen); err != nil {
			if !isNotFound(err) {
				return "", err
			}
			// Object not found.
			return fmt.Sprintf("<Change><ServerId>%v</ServerId><Status>8</Status></Change>", r.change.ServerId), nil
		}
	}
	output = fmt.Sprintf("<Change><ServerId>%v</ServerId><Status>1</Status></Change>", r.change.ServerId)
	logger.Debug(fmt.Sprintf("Client-side Change: ServerId=%v, Seen=%v", r.change.ServerId, r.change.ApplicationData.Read))

	return output, nil
}

// NOTE:
// We don't have to add this email to the virtual table because it is already
// synced, and therefore already added to the virtual table.
func (r *clientChangeSyncer) syncFetch() (output string, err error) {
	emailID, err := splitEmailID(r.change.ServerId)
	if err != nil {
		return "", errBadRequest
	}
	value, err := r.emailManager.GetEmail(emailID, database.LockNone)
	if err != nil {
		if !isNotFound(err) {
			return "", err
		}
		// Object not found.
		return fmt.Sprintf("<Fetch><ServerId>%v</ServerId><Status>8</Status></Fetch>", r.change.ServerId), nil
	}
	e := &email{Email: value, options: r.collection.Options, manager: r.emailManager}
	data, err := xml.Marshal(e)
	if err != nil {
		return "", err
	}
	output = fmt.Sprintf("<Fetch><ServerId>%v</ServerId><Status>1</Status>%v</Fetch>", r.change.ServerId, string(data))
	logger.Debug(fmt.Sprintf("Client-side Fetch: ServerId=%v", r.change.ServerId))

	return output, nil
}

func (r *handler) initialSync(sync activesync.Sync, manager backend.EmailManager, collection SyncCollection, resp *syncResp) error {
	logger.Debug(fmt.Sprintf("Initial email synchronizing: IP=%v, UserUID=%v, DeviceID=%v", r.req.RemoteAddr, sync.UserUID(), sync.DeviceID()))

	if collection.HasGetChanges() == true {
		logger.Debug("Initial email sync request has the GetChanges tag, which is a protocol error!")
		// Protocol error. GetChanges should be false if the SyncKey is 0.
		resp.status = 4
		return nil
	}

	if err := sync.ClearSyncKeys(); err != nil {
		return err
	}
	logger.Debug("Cleared the email sync key table!")
	if err := sync.ClearVirtualEmails(); err != nil {
		return err
	}
	logger.Debug("Cleared the virtual email table!")

	// Use a read lock to preserve the last history entry until we create a new sync key based on it.
	lastHistory, err := manager.GetEmailHistories(0, 1, true, database.LockRead)
	if err != nil {
		return err
	}
	logger.Debug(fmt.Sprintf("Last history ID = %v", lastHistory))

	var newSyncKey uint64
	if len(lastHistory) == 0 {
		newSyncKey, err = sync.NewSyncKey(0)
	} else {
		newSyncKey, err = sync.NewSyncKey(lastHistory[0].ID())
	}
	if err != nil {
		return err
	}
	logger.Debug(fmt.Sprintf("New SyncKey = %v", newSyncKey))

	resp.status = 1
	resp.syncKey = newSyncKey

	return nil
}

func (r *handler) sync(sync activesync.Sync, manager backend.EmailManager, collection SyncCollection, req *SyncReq, resp *syncResp) error {
	logger.Debug(fmt.Sprintf("Email synchronizing: IP=%v, UserUID=%v, DeviceID=%v, SyncKey=%v", r.req.RemoteAddr, sync.UserUID(), sync.DeviceID(), collection.SyncKey))

	// Use a write lock to make sure we sequentially process concurrent requests that have same sync key.
	historyID, err := sync.LoadSyncKey(collection.SyncKey, database.LockWrite)
	if err != nil {
		if !isNotFound(err) {
			return err
		}
		logger.Error(fmt.Sprintf("Client sent unknown email sync key: IP=%v, UserUID=%v, DeviceID=%v, SyncKey=%v", r.req.RemoteAddr, sync.UserUID(), sync.DeviceID(), collection.SyncKey))
		// Ask fullsync
		resp.status = 3
		resp.syncKey = 0
		return nil
	}
	logger.Debug(fmt.Sprintf("History ID from the SyncKey %v = %v", collection.SyncKey, historyID))

	lastSyncKey, err := getLastSyncKey(sync)
	if err != nil {
		return err
	}
	// Does client send the previous syncKey that is already processed before?
	if lastSyncKey != collection.SyncKey {
		logger.Error(fmt.Sprintf("Client sent corrupted email sync key: IP=%v, UserUID=%v, DeviceID=%v, lastSyncKey=%v, sentSyncKey=%v", r.req.RemoteAddr, sync.UserUID(), sync.DeviceID(), lastSyncKey, collection.SyncKey))
		// Send the last SyncKey we assigned. The response will have responses
		// for client-side changes (i.e., FETCH) if they are requested.
		resp.status = 1
		resp.syncKey = lastSyncKey
		return nil
	}

	lastHistory, err := manager.GetEmailHistories(0, 1, true, database.LockNone)
	if err != nil {
		return err
	}
	// The account does not have any email (initially created) OR the client does not want to receive server-side changes?
	if len(lastHistory) == 0 || collection.HasGetChanges() == false {
		logger.Debug("The account does not have any email (initially created) or the client does not want to receive server-side changes..")
		resp.status = 1
		// No changes. Use same syncKey the client sent.
		resp.syncKey = collection.SyncKey
		if req.HasClientChanges() {
			// Update syncKey because the client sent client-side changes, but historyID should not be changed
			// because we may have server-side changes do not yet synced to the client.
			newSyncKey, err := sync.NewSyncKey(historyID)
			if err != nil {
				return err
			}
			resp.syncKey = newSyncKey
		}
		logger.Debug(fmt.Sprintf("New SyncKey = %v", resp.syncKey))
		return nil
	}
	logger.Debug(fmt.Sprintf("Last History ID of the folder %v = %v", manager.FolderID(), lastHistory[0].ID()))

	windowSize := maxSyncWindowSize
	if collection.WindowSize > 0 && collection.WindowSize < maxSyncWindowSize {
		windowSize = collection.WindowSize
	}
	// TODO: Decrease windowSize if MIMESupport is not 0 and MIMETruncation is no truncation.
	logger.Debug(fmt.Sprintf("windowSize = %v", windowSize))

	lastEmailID, err := getLastEmailID(sync)
	if err != nil {
		return err
	}
	logger.Debug(fmt.Sprintf("lastEmailID = %v", lastEmailID))

	// 0: Empty virtual table (special value), 1: No more email (reached the last one), 1<: We may have more emails.
	if lastEmailID != 1 {
		nextEmailID := lastEmailID
		if nextEmailID > 0 {
			nextEmailID -= 1
		}
		// NOTE: We rely on that the PING command should immediately return if there is pending histories.
		emails, err := r.getEmails(manager, nextEmailID, windowSize, collection.Options)
		if err != nil {
			return err
		}
		logger.Debug(fmt.Sprintf("# of candidate emails to be synced: %v", len(emails)))
		if len(emails) > 0 {
			logger.Debug(fmt.Sprintf("Syncing %v emails..", len(emails)))
			return r.syncEmails(sync, manager, emails, windowSize, historyID, resp)
		}
	}

	// No more histories that do not yet synced?
	// NOTE: We rely on that the PING command should return immediately if there is an item to be soft-deleted.
	if historyID == lastHistory[0].ID() {
		logger.Debug("Soft-deleting old emails..")
		return r.syncSoftDeletes(sync, collection, windowSize, historyID, resp)
	}

	logger.Debug("Syncing pending histories..")
	return r.syncPendingHistories(sync, manager, historyID, collection.Options, windowSize, resp)
}

func (r *handler) syncPendingHistories(sync activesync.Sync, manager backend.EmailManager, historyID uint64, options SyncOptions, windowSize int, resp *syncResp) error {
	// Use a read lock to preserve the histories until we create virtual table entries.
	histories, err := manager.GetEmailHistories(historyID+1, maxQueryRows, false, database.LockRead)
	if err != nil {
		return err
	}

	lastID := historyID
	moreAvail := false
	threshold := getTimeFilter(options.FilterType)
	ops := []string{}
	for i, hist := range histories {
		lastID = hist.ID()
		email, err := hist.Value()
		if err != nil {
			return err
		}
		syncer := &historySyncer{
			historyID: hist.ID(),
			sync:      sync,
			manager:   manager,
			options:   options,
			email:     email,
			threshold: threshold,
		}

		var o string
		switch hist.Operation() {
		case backend.EmailAdd:
			o, err = syncer.syncAdd()
		case backend.EmailDelete:
			o, err = syncer.syncDelete()
		case backend.EmailUpdateSeen:
			o, err = syncer.syncUpdateSeen()
		default:
			panic("Unexpected backend operation type")
		}
		if err != nil {
			return err
		}
		if len(o) > 0 {
			ops = append(ops, o)
		}

		// NOTE:
		// DO NOT OVER EXECUTE MORE THAN WINDOW SIZE. ABOVE ROUTINES ARE
		// ACTUALLY INSERT ENTRIES INTO THE VIRTUAL TABLES. SO, OVER EXECUTION
		// WITH CUTTING LAST ONE APPROACH WILL RAISE AN UNEXPECTED RESULT!
		if len(ops) == windowSize {
			// NOTE:
			// This naive approach may cause a problem because we set
			// the MoreAvailables field in the response, but maybe we
			// don't have any changes to be synced in the subsequent
			// Sync response by skipping all useless histroies.
			if i < len(histories)-1 {
				moreAvail = true
			}
			break
		}
	}

	// NOTE:
	// We should assign new SyncKey even if the response is empty by skipping
	// all the histories because historyID should be changed to lastID.
	newSyncKey, err := sync.NewSyncKey(lastID)
	if err != nil {
		return err
	}
	resp.status = 1
	resp.syncKey = newSyncKey
	// We may return empty sync response that does not have any changes when
	// the all pending histories are skipped. It is okay as a subsequent Ping
	// response will ask the client to sync again if we still have histories
	// to be synced in the folders.
	if len(ops) > 0 {
		resp.commands = strings.Join(ops, "")
		if moreAvail {
			resp.moreAvail = true
		}
	}
	logger.Debug(fmt.Sprintf("New SyncKey = %v (historyID=%v, # of ops=%v, moreAvail=%v)", newSyncKey, lastID, len(ops), resp.moreAvail))

	return nil
}

type historySyncer struct {
	historyID uint64
	sync      activesync.Sync
	manager   backend.EmailManager
	options   SyncOptions
	email     *backend.Email
	threshold time.Time
}

func (r *historySyncer) syncAdd() (output string, err error) {
	// Does the user still have this email?
	latest, err := r.manager.GetEmail(r.email.ID, database.LockRead)
	if err != nil {
		if !isNotFound(err) {
			return "", err
		}
		// Not found in the backend database. Skip this add history so that
		// skip all useless subsequent histories related to this one.
		logger.Debug(fmt.Sprintf("ADD: emailId=%v, skip because it does not exist in the backend database", r.email.ID))
		return "", nil
	}

	notFound := false
	_, err = r.sync.GetVirtualEmail(r.email.ID, database.LockWrite)
	if err != nil {
		if !isNotFound(err) {
			return "", err
		}
		// Not found in the virtual table.
		notFound = true
	}
	if r.email.Date.Before(r.threshold) || !notFound {
		logger.Debug(fmt.Sprintf("ADD: emailId=%v, skip because it is too old or already exists in the virtual table", r.email.ID))
		return "", nil
	}

	lastChange, err := getLastEmailHistoryID(r.manager, r.email.ID)
	if err != nil {
		return "", err
	}
	// Use the latest email data, instead of one from the history, to update
	// its flags to the latest values. This allows that useless subsequent
	// histories are automatically skipped by checking the virtual folder.
	if err := r.sync.AddVirtualEmail(latest, lastChange); err != nil {
		return "", err
	}
	e := &email{Email: latest, options: r.options, manager: r.manager}
	data, err := xml.Marshal(e)
	if err != nil {
		return "", err
	}

	logger.Debug(fmt.Sprintf("Added: ServerID=%v:%v", r.sync.FolderID(), r.email.ID))
	return fmt.Sprintf("<Add><ServerId>%v:%v</ServerId>%v</Add>", r.sync.FolderID(), r.email.ID, string(data)), nil
}

func (r *historySyncer) syncDelete() (output string, err error) {
	notFound := false
	_, err = r.sync.GetVirtualEmail(r.email.ID, database.LockWrite)
	if err != nil {
		if !isNotFound(err) {
			return "", err
		}
		notFound = true
	}
	if notFound {
		logger.Debug(fmt.Sprintf("DELETE: emailId=%v, skip because it does not exist in the virtual table", r.email.ID))
		return "", nil
	}

	if err := r.sync.RemoveVirtualEmail(r.email.ID); err != nil {
		return "", err
	}

	logger.Debug(fmt.Sprintf("Deleted: ServerID=%v:%v", r.sync.FolderID(), r.email.ID))
	return fmt.Sprintf("<Delete><ServerId>%v:%v</ServerId></Delete>", r.sync.FolderID(), r.email.ID), nil
}

func (r *historySyncer) syncUpdateSeen() (output string, err error) {
	notFound := false
	virt, err := r.sync.GetVirtualEmail(r.email.ID, database.LockWrite)
	if err != nil {
		if !isNotFound(err) {
			return "", err
		}
		notFound = true
	}
	if notFound || virt.Seen == r.email.Seen || r.historyID <= virt.LastHistoryID {
		logger.Debug(fmt.Sprintf("UPDATE: emailId=%v, skip because it does not exist, has same value, or is already processed", r.email.ID))
		return "", nil
	}

	if err := r.sync.UpdateVirtualEmailSeen(r.email.ID, r.email.Seen); err != nil {
		return "", err
	}
	seen := 0
	if r.email.Seen == true {
		seen = 1
	}

	logger.Debug(fmt.Sprintf("Updated: ServerID=%v:%v, Seen=%v", r.sync.FolderID(), r.email.ID, seen))
	return fmt.Sprintf("<Change><ServerId>%v:%v</ServerId><ApplicationData><email:Read>%v</email:Read></ApplicationData></Change>", r.sync.FolderID(), r.email.ID, seen), nil
}

func (r *handler) syncSoftDeletes(sync activesync.Sync, collection SyncCollection, windowSize int, historyID uint64, resp *syncResp) error {
	// Check soft-delete items
	sd, err := sync.GetOldVirtualEmails(getTimeFilter(collection.Options.FilterType), uint(windowSize+1), database.LockWrite)
	if err != nil {
		return err
	}
	// No soft-deleted items?
	if len(sd) == 0 {
		// No sync key change
		resp.status = 1
		resp.syncKey = collection.SyncKey
		logger.Debug("No soft-deleted emails")
		return nil
	}

	moreAvail := false
	if len(sd) == windowSize+1 {
		moreAvail = true
		// Cut the last one
		sd = sd[:len(sd)-1]
	}
	logger.Debug(fmt.Sprintf("Found %v old emails to be soft-deleted", len(sd)))

	// Remove the soft-deleting emails from the virtual email folder
	output := ""
	for _, v := range sd {
		if err := sync.RemoveVirtualEmail(v.ID); err != nil {
			return err
		}
		output += fmt.Sprintf("<SoftDelete><ServerId>%v:%v</ServerId></SoftDelete>", sync.FolderID(), v.ID)
		logger.Debug(fmt.Sprintf("Soft-deleted: ServerID=%v:%v", sync.FolderID(), v.ID))
	}

	// No change of historyID
	newSyncKey, err := sync.NewSyncKey(historyID)
	if err != nil {
		return err
	}
	resp.status = 1
	resp.syncKey = newSyncKey
	resp.commands = output
	if moreAvail {
		resp.moreAvail = true
	}
	logger.Debug(fmt.Sprintf("New SyncKey = %v (historyID=%v, moreAvail=%v)", newSyncKey, historyID, resp.moreAvail))

	return nil
}

// getLastEmailHistoryID returns the last history's ID related with emailID.
// It will return 0 if there is no hisotry about the email.
func getLastEmailHistoryID(manager backend.EmailManager, emailID uint64) (uint64, error) {
	h, err := manager.GetLastEmailHistory(emailID, database.LockRead)
	if err != nil {
		if !isNotFound(err) {
			return 0, err
		}
		// Not found
		return 0, nil
	}

	return h.ID(), nil
}

func (r *handler) syncEmails(sync activesync.Sync, manager backend.EmailManager, emails []*email, windowSize int, historyID uint64, resp *syncResp) error {
	moreAvail := false
	if len(emails) == windowSize+1 {
		moreAvail = true
		// Cut the last one
		emails = emails[:len(emails)-1]
		logger.Debug("Cut the last one of the emails to be synced")
	}

	// Add emails to the virtual folder
	var output bytes.Buffer
	for _, v := range emails {
		lastChange, err := getLastEmailHistoryID(manager, v.ID)
		if err != nil {
			return err
		}
		if err := sync.AddVirtualEmail(v.Email, lastChange); err != nil {
			if !isDuplicated(err) {
				return err
			}
			logger.Debug(fmt.Sprintf("Ignored the duplicated virtual email: %+v", v.Email))
		}
		data, err := xml.Marshal(v)
		if err != nil {
			return err
		}
		output.WriteString(fmt.Sprintf("<Add><ServerId>%v:%v</ServerId>%v</Add>", sync.FolderID(), v.ID, string(data)))
		logger.Debug(fmt.Sprintf("Added: ServerID=%v:%v", sync.FolderID(), v.ID))
	}

	// No change of historyID
	newSyncKey, err := sync.NewSyncKey(historyID)
	if err != nil {
		return err
	}
	resp.status = 1
	resp.syncKey = newSyncKey
	resp.commands = output.String()
	if moreAvail {
		resp.moreAvail = true
	}
	logger.Debug(fmt.Sprintf("New SyncKey = %v (historyID=%v, moreAvail=%v)", newSyncKey, historyID, resp.moreAvail))

	return nil
}

func (r *handler) getEmails(manager backend.EmailManager, nextEmailID uint64, windowSize int, options SyncOptions) ([]*email, error) {
	emails, err := manager.GetEmails(nextEmailID, uint64(windowSize)+1, true, database.LockRead)
	if err != nil {
		return nil, err
	}

	timeFilter := getTimeFilter(options.FilterType)
	result := make([]*email, 0)
	for _, v := range emails {
		if v.Date.Before(timeFilter) {
			continue
		}
		result = append(result, &email{Email: v, options: options, manager: manager})
	}

	return result, nil
}

func getTimeFilter(filterType string) time.Time {
	switch filterType {
	case "1": // 1 day
		return time.Now().AddDate(0, 0, -1)
	case "2": // 3 days
		return time.Now().AddDate(0, 0, -3)
	case "3": // 1 week
		return time.Now().AddDate(0, 0, -7)
	case "4": // 2 weeks
		return time.Now().AddDate(0, 0, -14)
	case "5": // 1 month
		return time.Now().AddDate(0, -1, 0)
	default: // All items
		return time.Time{} // Zero value means January 1, year 1, 00:00:00.000000000 UTC
	}
}

func getLastSyncKey(sync activesync.Sync) (lastSyncKey uint64, err error) {
	var ok bool
	lastSyncKey, ok, err = sync.GetLastSyncKey(database.LockNone)
	if err != nil {
		return 0, err
	}
	if !ok {
		// Empty sync key table
		lastSyncKey = 0
	}

	return lastSyncKey, nil
}

func getLastEmailID(sync activesync.Sync) (lastEmailID uint64, err error) {
	oldest, ok, err := sync.GetOldestVirtualEmail(database.LockNone)
	if err != nil {
		return 0, err
	}

	if !ok {
		lastEmailID = 0 // not found
	} else {
		if oldest.ID == 0 {
			panic("Invalid the oldest email's ID")
		}
		lastEmailID = oldest.ID
	}

	return lastEmailID, nil
}
