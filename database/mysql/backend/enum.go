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

import "github.com/superkkt/omega/backend"

func ConvToBackendFolderType(t string) backend.FolderType {
	switch t {
	case "INBOX":
		return backend.EmailInbox
	case "DRAFT":
		return backend.EmailDraft
	case "TRASH":
		return backend.EmailTrash
	case "SENT":
		return backend.EmailSent
	default:
		return backend.EmailFolder
	}
}

func ConvToFolderTypeString(t backend.FolderType) string {
	switch t {
	case backend.EmailInbox:
		return "INBOX"
	case backend.EmailDraft:
		return "DRAFT"
	case backend.EmailTrash:
		return "TRASH"
	case backend.EmailSent:
		return "SENT"
	case backend.EmailFolder:
		return "FOLDER"
	default:
		panic("Invalid folder type")
	}
}

func ConvToBackendFolderOperation(t string) backend.FolderOperation {
	switch t {
	case "ADD":
		return backend.FolderAdd
	case "DEL":
		return backend.FolderDelete
	case "UPDATE":
		return backend.FolderUpdate
	default:
		panic("Invalid folder operation")
	}
}

func ConvToFolderOperationString(t backend.FolderOperation) string {
	switch t {
	case backend.FolderAdd:
		return "ADD"
	case backend.FolderDelete:
		return "DEL"
	case backend.FolderUpdate:
		return "UPDATE"
	default:
		panic("Invalid folder operation")
	}
}

func ConvToBackendEmailOperation(t string) backend.EmailOperation {
	switch t {
	case "ADD":
		return backend.EmailAdd
	case "DEL":
		return backend.EmailDelete
	default:
		return backend.EmailUpdateSeen
	}
}

func ConvToEmailOperationString(t backend.EmailOperation) string {
	switch t {
	case backend.EmailAdd:
		return "ADD"
	case backend.EmailDelete:
		return "DEL"
	default:
		return "SEEN"
	}
}
