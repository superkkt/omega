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
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/Muzikatoshi/omega/backend"
	"github.com/Muzikatoshi/omega/database"

	"github.com/superkkt/logger"
)

var (
	timeoutDuration   = 30 * time.Second
	deadlineDuration  = 1 * time.Minute
	keepAliveDuration = 1 * time.Minute
	SMTPHost          = "127.0.0.1"
	SMTPPort          = 25
)

func (r *handler) handleSendMail(tx database.Transaction) error {
	req, err := r.parseSendmailRequest()
	if err != nil {
		return err
	}
	logger.Debug(fmt.Sprintf("Sendmail request: %+v", req))

	if req.saveInSent {
		email, err := r.saveSentEmail(tx, req.body.norm)
		if err != nil {
			return err
		}
		logger.Debug(fmt.Sprintf("Stored a new sent email: ID=%v", email.ID))
	}

	logger.Debug("Sending an outgoing email..")
	return r.param.Mailer.Send(r.credential.UserID(), req.body.rcpts, req.body.norm)
}

type sendmailReq struct {
	contentType string
	saveInSent  bool
	body        *mimeMsg
}

func (r *handler) parseSendmailRequest() (sendmailReq, error) {
	out := sendmailReq{}

	saveInSent := r.req.URL.Query().Get("SaveInSent")
	switch saveInSent {
	case "T":
		out.saveInSent = true
	case "F":
		out.saveInSent = false
	default:
		r.badRequest = true
		return sendmailReq{}, fmt.Errorf("invalid SaveInSent URI parameter: %v", saveInSent)
	}

	out.contentType = strings.ToLower(r.req.Header.Get("Content-Type"))
	if out.contentType != "message/rfc822" {
		r.badRequest = true
		return sendmailReq{}, fmt.Errorf("invalid Content-Type value: %v", out.contentType)
	}

	data, err := ioutil.ReadAll(r.req.Body)
	if err != nil {
		return sendmailReq{}, err
	}
	msg, err := newMIMEMsg(data)
	if err != nil {
		return sendmailReq{}, err
	}
	out.body = msg

	return out, nil
}

func (r *handler) saveSentEmail(tx database.Transaction, msg []byte) (*backend.Email, error) {
	fm := r.param.BackendStorage.NewFolderManager(tx, r.credential)
	sent, err := fm.GetFolderByType(backend.EmailSent, database.LockRead)
	if err != nil {
		return nil, err
	}
	if len(sent) == 0 {
		return nil, errors.New("not found a sent item folder")
	}

	em := r.param.BackendStorage.NewEmailManager(tx, r.credential, sent[0].ID)
	email, err := em.AddEmail(msg)
	if err != nil {
		return nil, fmt.Errorf("AddEmail: %v", err)
	}
	if err := em.UpdateEmail(email.ID, true); err != nil {
		return nil, fmt.Errorf("UpdateEmail: %v", err)
	}

	return email, nil
}
