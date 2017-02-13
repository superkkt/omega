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

import "github.com/Muzikatoshi/omega/activesync"

func NewFactory() activesync.Factory {
	return new(factory)
}

type factory struct{}

func (r *factory) New(param activesync.Parameter) activesync.Handler {
	return &handler{
		param: param,
	}
}

func (r *factory) Version() string {
	return "2.5"
}

func (r *factory) Commands() []string {
	return []string{
		"FolderCreate", "FolderDelete", "FolderUpdate", "Provision", "FolderSync",
		"Sync", "Ping", "GetAttachment", "GetHierarchy", "GetItemEstimate", "MoveItems",
		"ResolveRecipeints", "SendMail", "SmartForward", "SmartReply", "MeetingResponse",
		"Search", "ValidateCert"}
}
