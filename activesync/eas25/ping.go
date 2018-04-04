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
	"time"

	"github.com/superkkt/omega/activesync"
	"github.com/superkkt/omega/database"

	"github.com/superkkt/logger"
)

const (
	maxPingFolders       = 20
	minHeartbeatInterval = 60  // Sec
	maxHeartbeatInterval = 600 // Sec
	pollInterval         = 15  // Sec
)

var (
	// FIXME: This cache should be stored on a shared storage for load-balancing.
	pingReqCache map[string]*PingReq = map[string]*PingReq{}
)

type PingReq struct {
	XMLName           xml.Name `xml:"Ping"`
	HeartbeatInterval uint64
	Folders           struct {
		Folder []struct {
			Id    uint64
			Class string
		}
	}
}

// TODO: Implement the event-driven direct push using Redis Pub/Sub!

// TODO: Notify if there are items to be soft-deleted.

// NOTE:
// Ping should immediately return if there are histories to be synced in the specified folders.
//
// BE CAREFUL IF YOU USE A DATABASE READ OR WRITE LOCK IN THIS FUNCTION. IT MAY
// BLOCK OTHER REQUESTS UNTIL AWAKE FROM THE SLEEP IN THIS FUNCTION.
func (r *handler) handlePing(tx database.Transaction) error {
	// Ping response is given in WBXML encoding.
	r.resp.SetWBXML(true)

	reqBody := new(PingReq)
	if err := activesync.ParseWBXMLRequest(r.req, reqBody); err != nil {
		r.badRequest = true
		return fmt.Errorf("ParseWBXMLRequest: %v", err)
	}
	logger.Debug(fmt.Sprintf("Ping request: %+v", reqBody))

	resp, err := r.ping(tx, reqBody)
	if err != nil {
		return err
	}
	// Default XML namespace
	resp.NS = "Ping:"

	output, err := xml.Marshal(resp)
	if err != nil {
		return err
	}
	r.resp.Write(output)

	return nil
}

func (r *handler) ping(tx database.Transaction, reqBody *PingReq) (PingResp, error) {
	// No parameters in the Ping request?
	if len(reqBody.Folders.Folder) == 0 || reqBody.HeartbeatInterval == 0 {
		v, ok := loadCachedPingReq(r.credential.UserUID(), getDeviceID(r.req))
		// Do we have any previous Ping request in the cache?
		if !ok {
			logger.Debug(fmt.Sprintf("Invalid Ping request: # of folders = %v, HeartbeatInterval = %v", len(reqBody.Folders.Folder), reqBody.HeartbeatInterval))
			// Ask to reissue the Ping command request with the entire XML body.
			return PingResp{Status: 3}, nil
		}
		logger.Debug(fmt.Sprintf("Loaded the Ping request cache: %+v", v))

		// Update the cached parameters if they are exist in this request.
		if len(reqBody.Folders.Folder) > 0 {
			v.Folders.Folder = reqBody.Folders.Folder
			logger.Debug(fmt.Sprintf("Updated the Ping request cache: Folders=%+v", v.Folders.Folder))
		}
		if reqBody.HeartbeatInterval != 0 {
			v.HeartbeatInterval = reqBody.HeartbeatInterval
			logger.Debug(fmt.Sprintf("Updated the Ping request cache: HeartbeatInterval=%v", v.HeartbeatInterval))
		}
		reqBody = v
	}

	// Ask to resend the Ping command request with the new, shorter list if it is requested with too many folders.
	if len(reqBody.Folders.Folder) > maxPingFolders {
		logger.Debug(fmt.Sprintf("Too many monitoring folders in the Ping request: %v", len(reqBody.Folders.Folder)))
		return PingResp{Status: 6, MaxFolders: maxPingFolders}, nil
	}
	// Ask to resend the Ping command with adjusted heartbeat interval if it is outside the allowed range.
	if reqBody.HeartbeatInterval < minHeartbeatInterval {
		logger.Debug(fmt.Sprintf("HeartbeatInterval is too short: %v", reqBody.HeartbeatInterval))
		return PingResp{Status: 5, HeartbeatInterval: minHeartbeatInterval}, nil
	}
	if reqBody.HeartbeatInterval > maxHeartbeatInterval {
		logger.Debug(fmt.Sprintf("HeartbeatInterval is too long: %v", reqBody.HeartbeatInterval))
		return PingResp{Status: 5, HeartbeatInterval: maxHeartbeatInterval}, nil
	}

	// Store this ping request into the cache.
	cachePingReq(r.credential.UserUID(), getDeviceID(r.req), reqBody)
	deadline := time.Now().Add(time.Duration(reqBody.HeartbeatInterval) * time.Second)
again:
	changes := []uint64{}
	for _, v := range reqBody.Folders.Folder {
		fm := r.param.BackendStorage.NewFolderManager(tx, r.credential)
		// Check folder existence
		_, err := fm.GetFolderByID(v.Id, database.LockNone)
		if err != nil {
			if !isNotFound(err) {
				return PingResp{}, err
			}
			logger.Debug(fmt.Sprintf("Unknown folder ID in the Ping request: folderID=%v", v.Id))
			// The folder hierarchy is out of date.
			return PingResp{Status: 7}, nil
		}

		sync := r.param.ASStorage.NewSync(tx, r.credential.UserUID(), getDeviceID(r.req), v.Id)
		lastSyncKey, ok, err := sync.GetLastSyncKey(database.LockNone)
		if err != nil {
			return PingResp{}, err
		}
		// Empty syncKey table?
		if !ok {
			continue
		}
		historyID, err := sync.LoadSyncKey(lastSyncKey, database.LockNone)
		if err != nil {
			// Not found is also treated as an error because that condition is a logic error.
			return PingResp{}, err
		}

		em := r.param.BackendStorage.NewEmailManager(tx, r.credential, v.Id)
		histories, err := em.GetEmailHistories(0, 1, true, database.LockNone)
		if err != nil {
			return PingResp{}, err
		}
		if len(histories) > 0 && historyID != histories[0].ID() {
			changes = append(changes, v.Id)
		}
	}

	if len(changes) > 0 {
		// We have histories to be synced.
		logger.Debug(fmt.Sprintf("Ping founds %v changes: folder IDs=%+v", len(changes), changes))
		return PingResp{Status: 2, Folders: &ChangedFolder{Folder: changes}}, nil
	}

	gap := deadline.Sub(time.Now())
	// Deadline reached?
	if gap < 0 {
		logger.Debug("No changes during the Ping period!")
		// No changed folders to be synchronized
		return PingResp{Status: 1}, nil
	}
	if gap > time.Duration(pollInterval)*time.Second {
		gap = time.Duration(pollInterval) * time.Second
	}
	logger.Debug(fmt.Sprintf("Sleeping for %v..", gap))
	time.Sleep(gap)

	// Restart the transaction to see new DB records.
	if err := restartTx(tx); err != nil {
		return PingResp{}, err
	}
	goto again
}

func restartTx(tx database.Transaction) error {
	if err := tx.Commit(); err != nil {
		return err
	}
	// Begin will start a new fresh transaction on tx.
	return tx.Begin()
}

type PingResp struct {
	XMLName           xml.Name `xml:"Ping"`
	NS                string   `xml:"xmlns,attr"`
	Status            int
	HeartbeatInterval uint           `xml:",omitempty"`
	MaxFolders        uint           `xml:",omitempty"`
	Folders           *ChangedFolder `xml:",omitempty"`
}

type ChangedFolder struct {
	Folder []uint64
}

func cachePingReq(userUID uint64, deviceID string, req *PingReq) {
	key := fmt.Sprintf("%v:%v", userUID, deviceID)
	pingReqCache[key] = req
}

func loadCachedPingReq(userUID uint64, deviceID string) (req *PingReq, ok bool) {
	key := fmt.Sprintf("%v:%v", userUID, deviceID)
	v, ok := pingReqCache[key]
	return v, ok
}
