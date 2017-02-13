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
	"fmt"

	"github.com/Muzikatoshi/omega/activesync"
	"github.com/Muzikatoshi/omega/backend"
	"github.com/Muzikatoshi/omega/database"

	"github.com/superkkt/logger"
)

func (r *handler) handleMoveItems(tx database.Transaction) error {
	// MoveItems response is given in WBXML encoding.
	r.resp.SetWBXML(true)

	reqBody := struct {
		XMLName xml.Name `xml:"MoveItems"`
		Move    []MoveItem
	}{}

	if err := activesync.ParseWBXMLRequest(r.req, &reqBody); err != nil {
		r.badRequest = true
		return fmt.Errorf("ParseWBXMLRequest: %v", err)
	}
	logger.Debug(fmt.Sprintf("MoveItems request: %+v", reqBody))

	// Empty move items?
	if len(reqBody.Move) == 0 {
		r.badRequest = true
		return fmt.Errorf("empty MoveItems request: IP=%v, UserUID=%v, DeviceID=%v", r.req.RemoteAddr, r.credential.UserUID(), getDeviceID(r.req))
	}

	return r.moveItems(tx, reqBody.Move)
}

type MoveItem struct {
	SrcMsgId string
	SrcFldId uint64
	DstFldId uint64
}

type MoveItemsResp struct {
	XMLName  xml.Name `xml:"MoveItems"`
	NS       string   `xml:"xmlns,attr"`
	Response []Response
}

type Response struct {
	SrcMsgId string
	Status   int
	DstMsgId string `xml:",omitempty"`
}

func (r *handler) moveItems(tx database.Transaction, items []MoveItem) error {
	if len(items) == 0 {
		panic("empty move items")
	}

	resp := &MoveItemsResp{
		NS: "Move:",
	}
	fm := r.param.BackendStorage.NewFolderManager(tx, r.credential)
	for _, v := range items {
		// Validate folder IDs
		if v.SrcFldId == v.DstFldId {
			// Source and destination collection IDs are the same.
			resp.Response = append(resp.Response, Response{SrcMsgId: v.SrcMsgId, Status: 4})
			continue
		}
		// Check the source folder
		ok, err := r.isExistFolder(fm, v.SrcFldId)
		if err != nil {
			return err
		}
		if !ok {
			// We don't have the source folder.
			resp.Response = append(resp.Response, Response{SrcMsgId: v.SrcMsgId, Status: 1})
			continue
		}
		// Check the destination folder
		ok, err = r.isExistFolder(fm, v.DstFldId)
		if err != nil {
			return err
		}
		if !ok {
			// We don't have the destination folder.
			resp.Response = append(resp.Response, Response{SrcMsgId: v.SrcMsgId, Status: 2})
			continue
		}

		msgID, err := splitEmailID(v.SrcMsgId)
		if err != nil {
			r.badRequest = true
			return fmt.Errorf("invalid SrcMsgId: %v", v.SrcMsgId)
		}
		// NOTE:
		// DO NOT APPLY THIS CHANGE TO THE VIRTUAL TABLE SO THAT A NEXT SYNC REQUEST
		// RECEIVES THIS CHANGE HISTORY!!!
		newMsgID, err := r.param.BackendStorage.NewEmailManager(tx, r.credential, v.SrcFldId).MoveEmail(msgID, v.DstFldId)
		if err != nil {
			if !isNotFound(err) {
				return err
			}
			// We don't have the email which is being moved.
			resp.Response = append(resp.Response, Response{SrcMsgId: v.SrcMsgId, Status: 1})
			continue
		}

		// Success
		dstMsgID := fmt.Sprintf("%v:%v", v.DstFldId, newMsgID)
		resp.Response = append(resp.Response, Response{SrcMsgId: v.SrcMsgId, Status: 3, DstMsgId: dstMsgID})
	}

	output, err := xml.Marshal(resp)
	if err != nil {
		return err
	}
	r.resp.Write(output)

	return nil
}

func (r *handler) isExistFolder(manager backend.FolderManager, folderID uint64) (ok bool, err error) {
	// Use the read lock to preserve the folder until the move is finished.
	_, err = manager.GetFolderByID(folderID, database.LockRead)
	if err != nil {
		if !isNotFound(err) {
			return false, err
		}
		// Not found
		return false, nil
	}

	return true, nil
}
