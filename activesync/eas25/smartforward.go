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
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"strconv"
	"strings"

	"github.com/superkkt/omega/database"

	"github.com/jhillyerd/go.enmime"
	"github.com/superkkt/logger"
)

func (r *handler) handleSmartForward(tx database.Transaction) error {
	req, err := r.parseSmartForwardRequest()
	if err != nil {
		return err
	}
	logger.Debug(fmt.Sprintf("SmartForward request: %+v", req))

	return r.forward(tx, req)
}

type smartForwardReq struct {
	contentType  string
	collectionID uint64
	itemID       uint64
	saveInSent   bool
	body         *mimeMsg
}

func (r *handler) parseSmartForwardRequest() (smartForwardReq, error) {
	out := smartForwardReq{}

	saveInSent := r.req.URL.Query().Get("SaveInSent")
	switch saveInSent {
	case "T":
		out.saveInSent = true
	case "F":
		out.saveInSent = false
	default:
		r.badRequest = true
		return smartForwardReq{}, fmt.Errorf("invalid SaveInSent URI parameter: %v", saveInSent)
	}

	out.contentType = strings.ToLower(r.req.Header.Get("Content-Type"))
	if out.contentType != "message/rfc822" {
		r.badRequest = true
		return smartForwardReq{}, fmt.Errorf("invalid Content-Type value: %v", out.contentType)
	}

	data, err := ioutil.ReadAll(r.req.Body)
	if err != nil {
		return smartForwardReq{}, err
	}
	msg, err := newMIMEMsg(data)
	if err != nil {
		return smartForwardReq{}, err
	}
	out.body = msg

	collectionID := r.req.URL.Query().Get("CollectionId")
	out.collectionID, err = strconv.ParseUint(collectionID, 10, 64)
	if err != nil {
		return smartForwardReq{}, err
	}

	out.itemID, err = splitEmailID(r.req.URL.Query().Get("ItemId"))
	if err != nil {
		return smartForwardReq{}, err
	}

	return out, nil
}

func (r *handler) forward(tx database.Transaction, req smartForwardReq) error {
	raw, err := r.getRawEmail(tx, req.collectionID, req.itemID)
	if err != nil {
		return err
	}
	logger.Debug(fmt.Sprintf("Fetched a raw email: size=%v", len(raw)))

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := addAlternativePart(w, req.body.parsed.mime); err != nil {
		return err
	}
	if err := addAttachmentPart(w, req.body.parsed.mime, raw); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	mime := []byte(fmt.Sprintf("%v\r\n%v", newHeader(req.body.parsed.header, w.Boundary()), buf.String()))
	logger.Debug(fmt.Sprintf("Reconstructed a new MIME message for SmartForward: size=%v", len(mime)))

	if err := r.param.Mailer.Send(r.credential.UserID(), req.body.rcpts, mime); err != nil {
		return err
	}

	// Do we need to save the send email into the "Sent Messages" folder?
	if !req.saveInSent {
		return nil
	}
	email, err := r.saveSentEmail(tx, mime)
	if err != nil {
		return err
	}
	logger.Debug(fmt.Sprintf("Stored a new SmartForward email: ID=%v", email.ID))

	return nil
}

func (r *handler) getRawEmail(tx database.Transaction, folderID uint64, emailID uint64) ([]byte, error) {
	logger.Debug(fmt.Sprintf("getRawEmail: folderID=%v, emailID=%v", folderID, emailID))

	// Check folder existence. Use a read lock to preserve the folder until we get the email.
	fm := r.param.BackendStorage.NewFolderManager(tx, r.credential)
	_, err := fm.GetFolderByID(folderID, database.LockRead)
	if err != nil {
		return nil, err
	}

	em := r.param.BackendStorage.NewEmailManager(tx, r.credential, folderID)
	return em.GetRawEmail(emailID, database.LockNone)
}

func newHeader(header mail.Header, boundary string) string {
	var out bytes.Buffer
	for k, v := range header {
		// Skip several useless fields
		low := strings.ToLower(k)
		if low == "bcc" || low == "content-type" || low == "mime-version" {
			continue
		}
		// Ignore a field that has no value
		if len(v) == 0 {
			continue
		}
		// FIXME: What should we do if there are multiple values?
		out.WriteString(fmt.Sprintf("%v: %v\r\n", k, v[0]))
	}
	out.WriteString("Content-Type: " + fmt.Sprintf("multipart/related;\r\n\tboundary=\"%v\";\r\n\ttype=\"multipart/alternative\"\r\n", boundary))
	out.WriteString("Mime-Version: 1.0\r\n")

	return out.String()
}

func addAttachmentPart(w *multipart.Writer, mime *enmime.MIMEBody, origMsg []byte) error {
	// Add new attachments of this SmartForward message.
	if err := addNewAttachs(w, mime.Attachments); err != nil {
		return err
	}
	// Add new inlines of this SmartForward message.
	if err := addNewAttachs(w, mime.Inlines); err != nil {
		return err
	}

	// Add the previous message as a RFC822 attachment.
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", "attachment; filename=\"MailAttachment.eml\"")
	header.Set("Content-Type", "message/rfc822")
	header.Set("Content-Transfer-Encoding", "base64")
	attach, err := w.CreatePart(header)
	if err != nil {
		return err
	}
	body := addLineBreak(base64.StdEncoding.EncodeToString(origMsg))
	if _, err := attach.Write([]byte(body)); err != nil {
		return err
	}

	return nil
}

func addNewAttachs(w *multipart.Writer, attachs []enmime.MIMEPart) error {
	for _, v := range attachs {
		attach, err := w.CreatePart(v.Header())
		if err != nil {
			return err
		}
		if _, err := attach.Write(v.RawContent()); err != nil {
			return err
		}
	}

	return nil
}

func addAlternativePart(w *multipart.Writer, mime *enmime.MIMEBody) error {
	var buf bytes.Buffer
	sub := multipart.NewWriter(&buf)
	if err := addTextPart(sub, mime); err != nil {
		return err
	}
	if err := addHTMLPart(sub, mime); err != nil {
		return err
	}
	if err := sub.Close(); err != nil {
		return err
	}

	header := textproto.MIMEHeader{}
	header.Set("Content-Type", fmt.Sprintf("multipart/alternative;\r\n\tboundary=\"%v\"", sub.Boundary()))
	alternative, err := w.CreatePart(header)
	if err != nil {
		return err
	}
	if _, err := alternative.Write(buf.Bytes()); err != nil {
		return err
	}

	return nil
}

func addTextPart(w *multipart.Writer, mime *enmime.MIMEBody) error {
	if len(mime.Text) == 0 {
		return nil
	}

	header := textproto.MIMEHeader{}
	header.Set("Content-Type", `text/plain; charset="utf-8"`)
	header.Set("Content-Transfer-Encoding", "base64")
	part, err := w.CreatePart(header)
	if err != nil {
		return err
	}
	body := addLineBreak(base64.StdEncoding.EncodeToString([]byte(mime.Text)))
	if _, err := part.Write([]byte(body)); err != nil {
		return err
	}

	return nil
}

func addHTMLPart(w *multipart.Writer, mime *enmime.MIMEBody) error {
	if len(mime.HTML) == 0 {
		return nil
	}

	header := textproto.MIMEHeader{}
	header.Set("Content-Type", `text/html; charset="utf-8"`)
	header.Set("Content-Transfer-Encoding", "base64")
	part, err := w.CreatePart(header)
	if err != nil {
		return err
	}
	body := addLineBreak(base64.StdEncoding.EncodeToString([]byte(mime.HTML)))
	if _, err := part.Write([]byte(body)); err != nil {
		return err
	}

	return nil
}
