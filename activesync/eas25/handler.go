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
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Muzikatoshi/omega/activesync"
	"github.com/Muzikatoshi/omega/backend"
	"github.com/Muzikatoshi/omega/database"

	"github.com/superkkt/logger"
)

const (
	maxDeadlockRetries = 5
	// The maximum number of rows that can be queried at a time. This value
	// should be equal to or greater than the maximum Sync WindowSize.
	maxQueryRows = maxSyncWindowSize * 2
)

var (
	random *rand.Rand = rand.New(&randomSource{src: rand.NewSource(time.Now().Unix())})
)

// randomSourece is safe for concurrent use by multiple goroutines.
type randomSource struct {
	sync.Mutex
	src rand.Source
}

func (r *randomSource) Int63() (n int64) {
	r.Lock()
	defer r.Unlock()

	return r.src.Int63()
}

func (r *randomSource) Seed(seed int64) {
	r.Lock()
	defer r.Unlock()

	r.src.Seed(seed)
}

type handler struct {
	param      activesync.Parameter
	credential backend.Credential
	req        *http.Request
	resp       *activesync.ResponseWriter
	badRequest bool // A client sent a bad request?
}

func (r *handler) Handle(c backend.Credential, w http.ResponseWriter, req *http.Request) {
	r.credential = c
	r.req = req
	r.resp = activesync.NewResponseWriter(w)

	cmd := req.URL.Query().Get("Cmd")
	if cmd == "" {
		logger.Debug(fmt.Sprintf("Missing Cmd URI parameter from %v", req.RemoteAddr))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Missing Cmd URI parameter"))
		return
	}
	logger.Debug(fmt.Sprintf("CMD: %v", cmd))

	deadlockRetries := 0
	for {
		// Clear the buffered response to avoid duplication.
		r.resp.Clear()
		tx, err := r.newTransaction()
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to create a DB transaction: %v", err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// NOTE: tx will be committed or rollbacked in the handle method.
		err = r.handle(tx, cmd)
		if err == nil {
			// No error. Send the response to the client.
			if err := r.resp.Flush(); err != nil {
				logger.Error(fmt.Sprintf("Failed to flush the response writer: %v", err))
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}
		// We got an error!
		txErr := tx.Error()
		// Too many deadlocks or non-deadlock error?
		if txErr == nil || !txErr.IsDeadlock() || deadlockRetries >= maxDeadlockRetries {
			logger.Error(err.Error())
			if r.badRequest {
				w.WriteHeader(http.StatusBadRequest)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}

		// A deadlock occurrs, but we still have a retry chance.
		time.Sleep(time.Duration(random.Int31n(500)) * time.Millisecond)
		deadlockRetries++
		logger.Error(fmt.Sprintf("DB deadlock occurrs: deadlockRetries=%v", deadlockRetries))
	}
}

// handle processes an ActiveSync request and is responsible to commit or rollback the tx transaction.
// handle should return an error if we get a bad requst from the client or an internal server error.
// In the bad request situation, handle should set r.badRequest to true.
func (r *handler) handle(tx database.Transaction, cmd string) error {
	defer tx.Rollback()

	var err error
	switch strings.ToUpper(cmd) {
	case "PROVISION":
		err = r.handleProvision()
	case "FOLDERSYNC":
		err = r.handleFolderSync(tx)
	case "FOLDERCREATE":
		err = r.handleFolderCreate(tx)
	case "FOLDERDELETE":
		err = r.handleFolderDelete(tx)
	case "FOLDERUPDATE":
		err = r.handleFolderUpdate(tx)
	case "SYNC":
		err = r.handleSync(tx)
	case "GETATTACHMENT":
		err = r.handleGetAttachment(tx)
	case "PING":
		err = r.handlePing(tx)
	case "GETITEMESTIMATE":
		err = r.handleGetItemEstimate(tx)
	case "MOVEITEMS":
		err = r.handleMoveItems(tx)
	case "GETHIERARCHY":
		err = r.handleGetHierarchy(tx)
	case "SENDMAIL":
		err = r.handleSendMail(tx)
	// We handle both SmartForward and SmartReply using the handleSmartForward method.
	// So, a SmartReplied message has the previous email as an attachment it it.
	case "SMARTFORWARD", "SMARTREPLY":
		err = r.handleSmartForward(tx)
	default:
		logger.Debug(fmt.Sprintf("Unsupported command (%v) request", cmd))
		r.resp.WriteHeader(http.StatusNotImplemented)
		return nil
	}
	if err != nil {
		return err
	}

	return tx.Commit()
}

func getDeviceID(req *http.Request) string {
	id := req.URL.Query().Get("DeviceId")
	if id == "" {
		// We already checked the DeviceId URI parameter.
		panic("empty DeviceID in the HTTP request")
	}

	return id
}

func (r *handler) newTransaction() (database.Transaction, error) {
	tx := r.param.Transaction.NewTransaction()
	if err := tx.Begin(); err != nil {
		return nil, err
	}

	return tx, nil
}
