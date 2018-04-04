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

package backend

import (
	"bytes"
	"fmt"
	"net/mail"

	"github.com/superkkt/omega/backend"
	"github.com/superkkt/omega/database"

	"github.com/jhillyerd/go.enmime"
)

type Attachment struct {
	id          uint64
	emailID     uint64
	name        string
	contentType string
	contentID   string
	size        uint64
	method      string
	order       int
	manager     backend.EmailManager
}

func (r *Attachment) ID() uint64 {
	return r.id
}

func (r *Attachment) Name() string {
	return r.name
}

func (r *Attachment) ContentType() string {
	return r.contentType
}

func (r *Attachment) ContentID() string {
	return r.contentID
}

func (r *Attachment) Size() uint64 {
	return r.size
}

func (r *Attachment) IsInline() bool {
	return r.method == "INLINE"
}

func (r *Attachment) Value() ([]byte, error) {
	raw, err := r.manager.GetRawEmail(r.emailID, database.LockNone)
	if err != nil {
		return nil, err
	}

	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("read message: %v", err)
	}
	mime, err := enmime.ParseMIMEBody(msg)
	if err != nil {
		return nil, fmt.Errorf("parse mime body: %v", err)
	}

	switch r.method {
	case "NORMAL":
		if len(mime.Attachments) < r.order {
			return nil, fmt.Errorf("invalid attachments order: %v", r.order)
		}
		return mime.Attachments[r.order].Content(), nil
	case "INLINE":
		if len(mime.Inlines) < r.order {
			return nil, fmt.Errorf("invalid inlines order: %v", r.order)
		}
		return mime.Inlines[r.order].Content(), nil
	case "OTHER":
		if len(mime.OtherParts) < r.order {
			return nil, fmt.Errorf("invalid other parts order: %v", r.order)
		}
		return mime.OtherParts[r.order].Content(), nil
	default:
		return nil, fmt.Errorf("invalid method type: %v", r.method)
	}
}
