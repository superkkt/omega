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

package backend

import (
	"time"

	"github.com/Muzikatoshi/omega/database"
)

// NOTE: All methods of Storage should return database.TransactionError if an error occurrs.
type Storage interface {
	NewFolderManager(queryer database.Queryer, c Credential) FolderManager
	NewEmailManager(queryer database.Queryer, c Credential, folderID uint64) EmailManager
}

// FolderManager provides methods to fetch and manipulate user folders and
// their change histories.
type FolderManager interface {
	Credential() Credential

	// GetFolders returns all folders that belong to a user identified by
	// the credential. It is necessary to acquire a read or write lock
	// depending on the lock mode for the fetched folders to prevent any
	// concurrent updates from another transaction. GetFolders can return
	// nil if there is no folder.
	GetFolders(lock database.LockMode) (folders []Folder, err error)

	// GetFolderByID returns a folder that belong to a user identified by
	// the credential and its unique folder ID. It is necessary to acquire
	// a read or write lock depending on the lock mode for the fetched folder
	// to prevent any concurrent updates from another transaction.
	GetFolderByID(folderID uint64, lock database.LockMode) (folder Folder, err error)

	// GetFolderByPath returns a folder that belong to a user identified by
	// the credential and its path. An element of the path slice means a node on
	// the path. For example, "/a/b/c" should be represented as []string{"a",
	// "b", "c"}. It is necessary to acquire a read or write lock depending
	// on the lock mode for the fetched folder to prevent any concurrent
	// updates from another transaction.
	GetFolderByPath(path []string, lock database.LockMode) (folder Folder, err error)

	// GetFolderByType returns folders whose type is same with t. It is 
	// necessary to acquire a read or write lock depending on the lock
	// mode for the fetched folders to prevent any concurrent updates from
	// another transaction. GetFolderByType can return nil if there is no
	// folder that has been matched.
	GetFolderByType(t FolderType, lock database.LockMode) ([]Folder, error)

	// AddFolder adds a new folder under the parent folder whose ID is 
	// parentID. AddFolder should append a new folder history after it 
	// succeeds in adding the new folder.
	AddFolder(parentID uint64, name string, t FolderType) (folderID uint64, err error)

	// DeleteFolder removes a folder whose ID is folderID. DeleteFolder
	// should append a new folder history after it succeeds in removing
	// the new folder.
	DeleteFolder(folderID uint64) error

	// MoveFolder moves a folder, whose ID is folderID, under the target 
	// folder whose ID is newParentID. MoveFolder should append a new folder
	// history after it succeeds in moving the new folder.
	UpdateFolder(folderID, newParentID uint64, newName string) error

	// GetLastFolderHistory returns the last history related with folderID.
	GetLastFolderHistory(folderID uint64, lock database.LockMode) (FolderHistory, error)

	// GetFolderHistories returns folder change histories that belong to a 
	// user identified by the credential. It is necessary to acquire a read
	// or write lock depending on the lock mode for the fetched histories to
	// prevent any concurrent updates from another transaction. offset is a
	// folder history ID as a starting position of this query. desc means 
	// descending order of sort if it is true, otherwise ascending order. 
	// Zero offset means the last history if desc is true. Zero limit means 
	// no limit, which is infinite. GetFolderHistories can return nil if there 
	// is no change history.
	GetFolderHistories(offset, limit uint64, desc bool, lock database.LockMode) ([]FolderHistory, error)

	// DeleteFolderHistory removes a folder history whose ID is historyID.
	DeleteFolderHistory(historyID uint64) error
}

type Folder struct {
	ID       uint64 // Unique Identifier.
	Name     string 
	ParentID uint64 // Parent's Unique Identifier.
	Type     FolderType
}

type FolderType int

const (
	EmailInbox FolderType = iota
	EmailDraft
	EmailTrash
	EmailSent
	// Normal email folder
	EmailFolder
	// Emails waiting to be sent out will be placed in the OUTBOX, and after
	// its sent out, it would be in the SENT folder.
	EmailOutbox
)

type FolderHistory interface {
	ID() uint64 // History ID (NOT folder's ID).
	Operation() FolderOperation
	Value() (Folder, error)
	Timestamp() time.Time
}

type FolderOperation int

const (
	FolderAdd FolderOperation = iota
	FolderDelete
	FolderUpdate
)

type EmailManager interface {
	Credential() Credential
	
	// FolderID returns current folder's ID selected by this email manager.
	FolderID() uint64

	// GetEmails returns emails that belong to the folder identified by the
	// folderID. It is necessary to acquire a read or write lock depending on
	// the lock mode for the fetched emails to prevent any concurrent updates
	// from another transaction. offset is an email ID as a starting position 
	// of this query. desc means descending order of sort if it is true, 
	// otherwise ascending order. Zero offset means the last email if desc is 
	// true. Zero limit means no limit, which is infinite. GetEmails can return 
	// nil if there is no email we find.
	GetEmails(offset, limit uint64, desc bool, lock database.LockMode) (emails []*Email, err error)

	// GetEmail returns an email whose ID is emailID. It is necessary to
	// acquire a read or write lock depending on the lock mode for the
	// fetched email to prevent any concurrent updates from another
	// transaction.
	GetEmail(emailID uint64, lock database.LockMode) (email *Email, err error)

	// GetRawEmail returns a raw email whose ID is emailID. It is necessary
	// to acquire a read or write lock depending on the lock mode for the
	// fetched email to prevent any concurrent updates from another transaction.
	GetRawEmail(emailID uint64, lock database.LockMode) ([]byte, error)

	// GetAttachment returns an attachment whose ID is attID.
	GetAttachment(attID uint64) (Attachment, error)

	// AddEmail adds a new email and should also append a new email history
	// after it succeeds in adding the new mail.
	AddEmail(rawEmail []byte) (*Email, error)

	// UpdateEmail updates properties of an email whose ID is emailID.
	// UpdateEmail should append a new email history after it succeeds in
	// updating the properties.
	UpdateEmail(emailID uint64, seen bool) error

	// DeleteEmail removes an email whose ID is email ID. DeleteEmail
	// should append a new email history after it succeeds in removing
	// the email.
	DeleteEmail(emailID uint64) error

	// MoveEmail moves an email, whose ID is emailID, under the target folder 
	// whose ID is newFolderID. MoveEmail should append a new email history after
	// it succeeds in moving the email. This is a helper function to serialize
	// delete and subsequent add operations.
	MoveEmail(emailID, newFolderID uint64) (newEmailID uint64, err error)

	// GetNumEmailHistories returns the number of email histories whose email's
	// timestamp is within range from current time to (current time - duration). 
	// offset is an history ID as a starting position of this query. desc means 
	// descending order of sort if it is true, otherwise ascending order.
	GetNumEmailHistories(offset uint64, duration time.Duration, desc bool) (uint64, error)

	// GetLastEmailHistory returns the last history related with emailID. It is 
	// necessary to acquire a read or write lock depending on the lock mode for 
	// the fetched history to prevent any concurrent updates from another transaction.
	GetLastEmailHistory(emailID uint64, lock database.LockMode) (EmailHistory, error)

	// GetEmailHistories returns email change histories that belong to the folder
	// identified by the folder ID of this email manager. It is necessary to acquire
	// a read or write lock depending on the lock mode for the fetched histories
	// to prevent any concurrent updates from another transaction. offset is a
	// history ID as a starting position of this query. desc means descending order
	// of sort if it is true, otherwise ascending order. Zero offset means the last 
	// history if desc is true. Zero limit means no limit, which is infinite. 
	// GetEmailHistories can return nil if there is no change history.
	GetEmailHistories(offset, limit uint64, desc bool, lock database.LockMode) ([]EmailHistory, error)

	// DeleteEmailHistory removes an email change history whose ID is historyID.
	DeleteEmailHistory(historyID uint64) error
}

type Email struct {
	ID          uint64 // Unique Identifier.
	From        EmailAddress
	To          []EmailAddress
	ReplyTo     []EmailAddress
	Cc          []EmailAddress
	Subject     string
	Date        time.Time
	Body        string
	Charset     string // Content character set of the root MIME part, which can be empty string.
	Attachments []Attachment
	Seen        bool // Already read?
}

type EmailAddress struct {
	Name    string // i.e., Muzi Katoshi
	Address string // i.e., muzikatoshi@gmail.com
}

type Attachment interface {
	ID() uint64 // Unique Identifier.
	Name() string
	// ContentType returns MIME Content-Type.
	ContentType() string
	// ContentID returns MIME Content-ID without opening and closing parenthesis.
	ContentID() string
	// Size returns the length of decoded attachment data.
	Size() uint64
	// Value returns decoded attachment data.
	Value() ([]byte, error)
	IsInline() bool
}

type EmailHistory interface {
	ID() uint64 // History ID
	Operation() EmailOperation
	Value() (*Email, error)
	Timestamp() time.Time
}

type EmailOperation int

const (
	EmailAdd EmailOperation = iota
	EmailDelete
	EmailUpdateSeen
)
