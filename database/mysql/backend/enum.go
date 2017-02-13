package backend

import "github.com/Muzikatoshi/omega/backend"

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
