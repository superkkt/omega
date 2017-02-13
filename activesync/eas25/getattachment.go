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
	"strconv"
	"strings"

	"github.com/Muzikatoshi/omega/backend"
	"github.com/Muzikatoshi/omega/database"

	"github.com/superkkt/logger"
)

const (
	defaultContentType = "application/octet-stream"
)

func (r *handler) handleGetAttachment(tx database.Transaction) error {
	logger.Debug(fmt.Sprintf("GetAttachment request: %+v", r.req.URL.Query()))

	name := r.req.URL.Query().Get("AttachmentName")
	if name == "" {
		r.badRequest = true
		return errors.New("missing AttachmentName URI parameter")
	}

	folderID, attachID, err := splitAttachName(name)
	if err != nil {
		r.badRequest = true
		return fmt.Errorf("invalid AttachmentName value: %v", err)
	}
	manager := r.param.BackendStorage.NewEmailManager(tx, r.credential, folderID)

	return r.sendAttachment(manager, attachID)
}

func splitAttachName(name string) (folderID, attachID uint64, err error) {
	// name should be formatted as "folderID:attachID".
	t := strings.Split(name, ":")
	if len(t) != 2 {
		return 0, 0, fmt.Errorf("invalid AttachName format: %v", name)
	}

	val := make([]uint64, 2)
	for i, v := range t {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("non-numeric ID value: %v", v)
		}
		val[i] = n
	}

	// val[0]: folderID, val[1]: attachID
	return val[0], val[1], nil
}

func (r *handler) sendAttachment(manager backend.EmailManager, attachID uint64) error {
	a, err := manager.GetAttachment(attachID)
	if err != nil {
		if !isNotFound(err) {
			return err
		}
		// If the GetAttachment command is used to retrieve an attachment that has been
		// deleted on the server, a 500 status code is returned in the HTTP POST response.
		return fmt.Errorf("not found attachment: attachID=%v", attachID)
	}

	v, err := a.Value()
	if err != nil {
		return fmt.Errorf("failed to get attachment value: %v", err)
	}
	contentType := a.ContentType()
	if contentType == "" {
		contentType = defaultContentType
	}
	r.resp.Header().Set("Content-Type", contentType)
	r.resp.Header().Set("Content-Length", strconv.FormatUint(uint64(len(v)), 10))
	r.resp.Write(v)

	return nil
}
