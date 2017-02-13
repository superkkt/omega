package backend

import (
	"bytes"
	"database/sql"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/Muzikatoshi/omega/backend"
	"github.com/Muzikatoshi/omega/database"
	"github.com/Muzikatoshi/omega/database/mysql"

	"github.com/jhillyerd/go.enmime"
)

type EmailStorage struct {
	c        backend.Credential
	folderID uint64
	queryer  database.Queryer
	dbName   string
}

func (r *EmailStorage) Credential() backend.Credential {
	return r.c
}

func (r *EmailStorage) FolderID() uint64 {
	return r.folderID
}

func (r *EmailStorage) GetNumEmails(offset uint64, duration time.Duration, desc bool) (count uint64, err error) {
	f := func(tx *sql.Tx) error {
		qry := fmt.Sprintf("SELECT COUNT(*) FROM `%v`.`email` WHERE user_id = ? AND timestamp >= ? ", r.dbName)
		args := make([]interface{}, 0)
		args = append(args, r.c.UserUID(), time.Now().Add(-duration))
		if r.folderID != 0 {
			qry += "AND folder_id = ? "
			args = append(args, r.folderID)
		}
		if offset != 0 {
			if desc {
				qry += "AND `id` <= ?"
			} else {
				qry += "AND `id` >= ?"
			}
			args = append(args, offset)
		}

		if err := tx.QueryRow(qry, args...).Scan(&count); err != nil {
			return err
		}
		return nil
	}

	if err := r.queryer.Query(f); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *EmailStorage) GetEmails(offset, limit uint64, desc bool, lock database.LockMode) (emails []*backend.Email, err error) {
	f := func(tx *sql.Tx) error {
		qry := "SELECT `id`, `from`, `to`, `reply_to`, `cc`, `subject`, `body`, `charset`, `read`, `timestamp` "
		qry += fmt.Sprintf("FROM `%v`.`email` ", r.dbName)
		qry += "WHERE user_id = ? AND available = true "

		args := make([]interface{}, 0)
		args = append(args, r.c.UserUID())
		if r.folderID != 0 {
			qry += "AND folder_id = ? "
			args = append(args, r.folderID)
		}
		if offset != 0 {
			if desc {
				qry += "AND `id` <= ? "
			} else {
				qry += "AND `id` >= ? "
			}
			args = append(args, offset)
		}
		if desc {
			qry += "ORDER BY `id` DESC "
		} else {
			qry += "ORDER BY `id` ASC "
		}
		if limit != 0 {
			qry += "LIMIT ?"
			args = append(args, limit)
		}
		qry += mysql.GetLockCmd(lock)

		rows, err := tx.Query(qry, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		var ids []string
		for rows.Next() {
			var from, to string
			var replyTo, cc sql.NullString
			v := &backend.Email{}
			if err := rows.Scan(&v.ID, &from, &to, &replyTo, &cc, &v.Subject, &v.Body,
				&v.Charset, &v.Seen, &v.Date); err != nil {
				return err
			}
			v.From = parseAddress(from)
			v.To = parseAddressList(to)
			if replyTo.Valid {
				v.ReplyTo = parseAddressList(replyTo.String)
			}
			if cc.Valid {
				v.Cc = parseAddressList(cc.String)
			}
			emails = append(emails, v)
			ids = append(ids, strconv.FormatUint(v.ID, 10))
		}
		if len(ids) == 0 {
			return nil
		}

		// Get Attachment
		qry = "SELECT `id`, `email_id`, `content_type`, `content_id`, `name`, `size`, `method`, `order` "
		qry += "FROM " + r.dbName + ".`attachment` "
		qry += "WHERE `email_id` IN (" + strings.Join(ids, ", ") + ")"
		qry += mysql.GetLockCmd(lock)

		rows2, err := tx.Query(qry)
		if err != nil {
			return err
		}
		defer rows2.Close()
		for rows2.Next() {
			a := new(Attachment)
			a.manager = r
			if err := rows2.Scan(&a.id, &a.emailID, &a.contentType, &a.contentID,
				&a.name, &a.size, &a.method, &a.order); err != nil {
				return err
			}

			for _, v := range emails {
				if v.ID == a.emailID {
					v.Attachments = append(v.Attachments, a)
					break
				}
			}
		}
		return nil
	}

	if err := r.queryer.Query(f); err != nil {
		return nil, err
	}
	return emails, nil
}

func (r *EmailStorage) GetEmail(emailID uint64, lock database.LockMode) (v *backend.Email, err error) {
	f := func(tx *sql.Tx) error {
		// Get email
		qry := "SELECT `id`, `from`, `to`, `reply_to`, `cc`, `subject`, `body`, `charset`, `read`, `timestamp` "
		qry += "FROM " + r.dbName + ".`email` "
		qry += "WHERE `id` = ? AND `user_id` = ? AND `available` = TRUE "
		args := make([]interface{}, 0)
		args = append(args, emailID, r.c.UserUID())
		if r.folderID != 0 {
			qry += "AND folder_id = ? "
			args = append(args, r.folderID)
		}
		qry += mysql.GetLockCmd(lock)

		var from, to string
		var replyTo, cc sql.NullString
		v = new(backend.Email)
		if err := tx.QueryRow(qry, args...).Scan(&v.ID, &from, &to,
			&replyTo, &cc, &v.Subject, &v.Body, &v.Charset, &v.Seen, &v.Date); err != nil {
			return err
		}
		v.From = parseAddress(from)
		v.To = parseAddressList(to)
		if replyTo.Valid {
			v.ReplyTo = parseAddressList(replyTo.String)
		}
		if cc.Valid {
			v.Cc = parseAddressList(cc.String)
		}

		// Get Attachment
		qry = "SELECT `id`, `email_id`, `content_type`, `content_id`, `name`, `size`, `method`, `order` "
		qry += "FROM " + r.dbName + ".`attachment` "
		qry += "WHERE `email_id` = ?"
		qry += mysql.GetLockCmd(lock)

		rows, err := tx.Query(qry, emailID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			a := new(Attachment)
			a.manager = r
			if err := rows.Scan(&a.id, &a.emailID, &a.contentType, &a.contentID,
				&a.name, &a.size, &a.method, &a.order); err != nil {
				return err
			}
			v.Attachments = append(v.Attachments, a)
		}
		return nil
	}

	if err := r.queryer.Query(f); err != nil {
		return nil, err
	}
	return v, nil
}

func (r *EmailStorage) GetRawEmail(emailID uint64, lock database.LockMode) (v []byte, err error) {
	f := func(tx *sql.Tx) error {
		qry := "SELECT `data` FROM `" + r.dbName + "`.`raw_email` A "
		qry += "INNER JOIN `" + r.dbName + "`.`email` B "
		qry += "ON A.`email_id` = B.`id` "
		qry += "WHERE A.`email_id` = ? AND B.`user_id` = ? AND B.`available` = TRUE "
		args := make([]interface{}, 0)
		args = append(args, emailID, r.c.UserUID())
		if r.folderID != 0 {
			qry += "AND B.`folder_id` = ? "
			args = append(args, r.folderID)
		}
		qry += mysql.GetLockCmd(lock)

		if err := tx.QueryRow(qry, args...).Scan(&v); err != nil {
			return fmt.Errorf("GetRawEmail: emailID=%v, err=%v", emailID, err)
		}
		return nil
	}

	if err := r.queryer.Query(f); err != nil {
		return nil, err
	}
	return v, nil
}

func plainCharset(body string) string {
	startIndex := strings.Index(body, "\nContent-Type: text/plain; charset=")
	if startIndex == -1 {
		return ""
	}
	body = strings.TrimSpace(body[startIndex:])

	endIndex := strings.Index(body, "\n")
	if endIndex == -1 {
		return ""
	}
	body = strings.TrimSpace(body[:endIndex])

	return strings.Replace(strings.Split(body, "=")[1], "\"", "", -1)
}

func htmlCharset(body string) string {
	startIndex := strings.Index(body, "\nContent-Type: text/html; charset=")
	if startIndex == -1 {
		return ""
	}
	body = strings.TrimSpace(body[startIndex:])

	endIndex := strings.Index(body, "\n")
	if endIndex == -1 {
		return ""
	}
	body = strings.TrimSpace(body[:endIndex])

	return strings.Replace(strings.Split(body, "=")[1], "\"", "", -1)
}

// AddEmail should add a new email history.
func (r *EmailStorage) AddEmail(rawEmail []byte) (email *backend.Email, err error) {
	msg, err := mail.ReadMessage(bytes.NewReader(rawEmail)) // Read email using Go's net/mail
	if err != nil {
		return nil, fmt.Errorf("read email: %v", err)
	}
	m, err := enmime.ParseMIMEBody(msg) // Parse message body with enmime
	if err != nil {
		return nil, fmt.Errorf("parse mime body: %v", err)
	}
	charset := htmlCharset(string(rawEmail))
	if len(charset) == 0 {
		charset = plainCharset(string(rawEmail))
	}

	f := func(tx *sql.Tx) error {
		var qry string
		var result sql.Result
		// Add email
		if r.folderID == 0 {
			qry = "INSERT INTO `" + r.dbName + "`.`email`"
			qry += "(`user_id`, `from`, `to`, `reply_to`, `cc`, `subject`, `body`, `charset`) "
			qry += "VALUES(?, ?, ?, ?, ?, ?, ?, ?)"
			result, err = tx.Exec(qry, r.c.UserUID(), m.GetHeader("From"), m.GetHeader("To"),
				m.GetHeader("Reply-To"), m.GetHeader("CC"), m.GetHeader("Subject"), m.Text, charset)
		} else {
			qry = "INSERT INTO `" + r.dbName + "`.`email`"
			qry += "(`user_id`, `folder_id`, `from`, `to`, `reply_to`, `cc`, `subject`, `body`, `charset`) "
			qry += "VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)"
			result, err = tx.Exec(qry, r.c.UserUID(), r.folderID, m.GetHeader("From"), m.GetHeader("To"),
				m.GetHeader("Reply-To"), m.GetHeader("CC"), m.GetHeader("Subject"), m.Text, charset)
		}
		if err != nil {
			return err
		}
		emailID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		email = new(backend.Email)
		email.ID = uint64(emailID)

		// Add email history
		if r.folderID == 0 {
			qry = "INSERT INTO `" + r.dbName + "`.`email_history`"
			qry += "(`user_id`, `email_id`, `operation`, `read`) "
			qry += "VALUES(?, ?, 'ADD', false)"
			_, err = tx.Exec(qry, r.c.UserUID(), uint64(emailID))
		} else {
			qry = "INSERT INTO `" + r.dbName + "`.`email_history`"
			qry += "(`user_id`, `folder_id`, `email_id`, `operation`, `read`) "
			qry += "VALUES(?, ?, ?, 'ADD', false)"
			_, err = tx.Exec(qry, r.c.UserUID(), r.folderID, uint64(emailID))
		}
		if err != nil {
			return err
		}

		// Add raw email
		qry = "INSERT INTO `" + r.dbName + "`.`raw_email`(`email_id`, `data`) VALUE(?, ?)"
		if _, err := tx.Exec(qry, uint64(emailID), rawEmail); err != nil {
			return err
		}

		// Add email attachment
		attaches, err := r.insertMIMEParts(tx, m.Attachments, uint64(emailID), "NORMAL")
		if err != nil {
			return err
		}
		email.Attachments = append(email.Attachments, attaches...)

		attaches, err = r.insertMIMEParts(tx, m.Inlines, uint64(emailID), "INLINE")
		if err != nil {
			return err
		}
		email.Attachments = append(email.Attachments, attaches...)

		attaches, err = r.insertMIMEParts(tx, m.OtherParts, uint64(emailID), "OTHER")
		if err != nil {
			return err
		}
		email.Attachments = append(email.Attachments, attaches...)

		return nil
	}
	if err := r.queryer.Query(f); err != nil {
		return nil, err
	}

	email.From = parseAddress(m.GetHeader("From"))
	email.To = parseAddressList(m.GetHeader("To"))
	email.ReplyTo = parseAddressList(m.GetHeader("Reply-To"))
	email.Cc = parseAddressList(m.GetHeader("CC"))
	email.Subject = m.GetHeader("Subject")
	email.Date = time.Now()
	email.Body = m.Text
	email.Charset = charset
	email.Seen = false

	return email, nil
}

func (r *EmailStorage) insertMIMEParts(tx *sql.Tx, parts []enmime.MIMEPart, emailID uint64, method string) (attaches []backend.Attachment, err error) {
	for i, v := range parts {
		contentID := v.Header().Get("Content-Id")
		if len(contentID) == 0 {
			contentID = v.Header().Get("Content-ID")
		}
		// Remove '<' and '>'
		contentID = strings.Replace(contentID, "<", "", -1)
		contentID = strings.Replace(contentID, ">", "", -1)

		qry := "INSERT INTO `" + r.dbName + "`.`attachment`"
		qry += "(`email_id`, `content_type`, `content_id`, `name`, `size`, `method`, `order`) "
		qry += "VALUES(?, ?, ?, ?, ?, ?, ?)"
		result, err := tx.Exec(qry, emailID, v.ContentType(), contentID,
			v.FileName(), len(v.Content()), method, i)
		if err != nil {
			return nil, err
		}
		aid, err := result.LastInsertId()
		if err != nil {
			return nil, err
		}
		a := Attachment{
			id:          uint64(aid),
			emailID:     emailID,
			name:        v.FileName(),
			contentType: v.ContentType(),
			contentID:   contentID,
			size:        uint64(len(v.Content())),
			method:      method,
			order:       i,
			manager:     r,
		}
		attaches = append(attaches, &a)
	}

	return attaches, nil
}

// UpdateEmail should add a new email history.
func (r *EmailStorage) UpdateEmail(emailID uint64, read bool) error {
	f := func(tx *sql.Tx) error {
		qry := fmt.Sprintf("SELECT `id` FROM `%v`.`email` WHERE id = ? AND user_id = ? AND available = TRUE ", r.dbName)
		args := make([]interface{}, 0)
		args = append(args, emailID, r.c.UserUID())
		if r.folderID != 0 {
			qry += "AND folder_id = ? "
			args = append(args, r.folderID)
		}
		qry += "FOR UPDATE"

		var id uint64
		if err := tx.QueryRow(qry, args...).Scan(&id); err != nil {
			return err
		}

		qry = fmt.Sprintf("UPDATE `%v`.`email` SET `read` = ? WHERE id = ?", r.dbName)
		if _, err := tx.Exec(qry, read, emailID); err != nil {
			return err
		}

		var err error
		if r.folderID == 0 {
			qry = fmt.Sprintf("INSERT INTO `%v`.`email_history`", r.dbName)
			qry += "(`user_id`, `email_id`, `operation`, `read`) "
			qry += "VALUES(?, ?, 'SEEN', ?)"
			_, err = tx.Exec(qry, r.c.UserUID(), emailID, read)
		} else {
			qry = fmt.Sprintf("INSERT INTO `%v`.`email_history`", r.dbName)
			qry += "(`user_id`, `folder_id`, `email_id`, `operation`, `read`) "
			qry += "VALUES(?, ?, ?, 'SEEN', ?)"
			_, err = tx.Exec(qry, r.c.UserUID(), r.folderID, emailID, read)
		}
		if err != nil {
			return err
		}

		return nil
	}
	if err := r.queryer.Query(f); err != nil {
		return err
	}
	return nil
}

// DeleteEmail should add a new email history.
func (r *EmailStorage) DeleteEmail(emailID uint64) error {
	f := func(tx *sql.Tx) error {
		qry := fmt.Sprintf("SELECT `read` FROM `%v`.`email` WHERE id = ? AND user_id = ? AND available = TRUE ", r.dbName)
		args := make([]interface{}, 0)
		args = append(args, emailID, r.c.UserUID())
		if r.folderID != 0 {
			qry += "AND folder_id = ? "
			args = append(args, r.folderID)
		}
		qry += "FOR UPDATE"

		var read bool
		if err := tx.QueryRow(qry, args...).Scan(&read); err != nil {
			return err
		}

		qry = fmt.Sprintf("UPDATE `%v`.`email` SET available = FALSE WHERE id = ?", r.dbName)
		if _, err := tx.Exec(qry, emailID); err != nil {
			return err
		}

		var err error
		if r.folderID == 0 {
			qry = fmt.Sprintf("INSERT INTO `%v`.`email_history`", r.dbName)
			qry += "(`user_id`, `email_id`, `operation`, `read`) "
			qry += "VALUES(?, ?, 'DEL', ?)"
			_, err = tx.Exec(qry, r.c.UserUID(), emailID, read)
		} else {
			qry = fmt.Sprintf("INSERT INTO `%v`.`email_history`", r.dbName)
			qry += "(`user_id`, `folder_id`, `email_id`, `operation`, `read`) "
			qry += "VALUES(?, ?, ?, 'DEL', ?)"
			_, err = tx.Exec(qry, r.c.UserUID(), r.folderID, emailID, read)
		}
		if err != nil {
			return err
		}

		return nil
	}
	if err := r.queryer.Query(f); err != nil {
		return err
	}
	return nil
}

func (r *EmailStorage) MoveToTrash(emailID uint64) (newEmailID, trashFolderID uint64, err error) {
	f := func(tx *sql.Tx) error {
		qry := fmt.Sprintf("SELECT `read` FROM `%v`.`email` WHERE id = ? AND user_id = ? AND available = TRUE ", r.dbName)
		args := make([]interface{}, 0)
		args = append(args, emailID, r.c.UserUID())
		if r.folderID != 0 {
			qry += "AND folder_id = ? "
			args = append(args, r.folderID)
		}
		qry += "FOR UPDATE"

		var read bool
		if err := tx.QueryRow(qry, args...).Scan(&read); err != nil {
			return err
		}

		qry = fmt.Sprintf("SELECT `id` FROM `%v`.`folder` WHERE `user_id` = ? AND `type` = 'TRASH' ORDER BY `id` ASC LIMIT 1 LOCK IN SHARE MODE", r.dbName)
		if err := tx.QueryRow(qry, r.c.UserUID()).Scan(&trashFolderID); err != nil {
			return err
		}

		qry = fmt.Sprintf("UPDATE `%v`.`email` SET folder_id = ? WHERE id = ?", r.dbName)
		if _, err := tx.Exec(qry, trashFolderID, emailID); err != nil {
			return err
		}

		qry = fmt.Sprintf("INSERT INTO `%v`.`email_history`", r.dbName)
		qry += "(`user_id`, `folder_id`, `email_id`, `operation`, `read`) "
		qry += "VALUES(?, ?, ?, 'DEL', ?)"
		if _, err := tx.Exec(qry, r.c.UserUID(), r.folderID, emailID, read); err != nil {
			return err
		}

		qry = fmt.Sprintf("INSERT INTO `%v`.`email_history`", r.dbName)
		qry += "(`user_id`, `folder_id`, `email_id`, `operation`, `read`) "
		qry += "VALUES(?, ?, ?, 'ADD', ?)"
		if _, err := tx.Exec(qry, r.c.UserUID(), trashFolderID, emailID, read); err != nil {
			return err
		}

		return nil
	}
	if err := r.queryer.Query(f); err != nil {
		return 0, 0, err
	}
	return emailID, trashFolderID, nil
}

// MoveEmail should add a new email history.
// MoveEmail is a helper function to serialize delete and add functions.
func (r *EmailStorage) MoveEmail(emailID, newFolderID uint64) (newEmailID uint64, err error) {
	f := func(tx *sql.Tx) error {
		qry := fmt.Sprintf("SELECT `read` FROM `%v`.`email` WHERE id = ? AND user_id = ? ", r.dbName)
		args := make([]interface{}, 0)
		args = append(args, emailID, r.c.UserUID())
		if r.folderID != 0 {
			qry += "AND folder_id = ? "
			args = append(args, r.folderID)
		}
		qry += "FOR UPDATE"

		var read bool
		if err := tx.QueryRow(qry, args...).Scan(&read); err != nil {
			return err
		}

		qry = fmt.Sprintf("UPDATE `%v`.`email` SET folder_id = ? WHERE id = ?", r.dbName)
		if _, err := tx.Exec(qry, newFolderID, emailID); err != nil {
			return err
		}

		qry = fmt.Sprintf("INSERT INTO `%v`.`email_history`", r.dbName)
		qry += "(`user_id`, `folder_id`, `email_id`, `operation`, `read`) "
		qry += "VALUES(?, ?, ?, 'DEL', ?)"
		if _, err := tx.Exec(qry, r.c.UserUID(), r.folderID, emailID, read); err != nil {
			return err
		}

		qry = fmt.Sprintf("INSERT INTO `%v`.`email_history`", r.dbName)
		qry += "(`user_id`, `folder_id`, `email_id`, `operation`, `read`) "
		qry += "VALUES(?, ?, ?, 'ADD', ?)"
		if _, err := tx.Exec(qry, r.c.UserUID(), newFolderID, emailID, read); err != nil {
			return err
		}

		return nil
	}
	if err := r.queryer.Query(f); err != nil {
		return 0, err
	}
	return emailID, nil
}

// GetNumEmailHistories returns the number of email histories whose email's timestamp is
// within range from current time. offset is an history ID as a starting position.
// desc means descending order if it is true, otherwise ascending order.
func (r *EmailStorage) GetNumEmailHistories(offset uint64, duration time.Duration, desc bool) (count uint64, err error) {
	f := func(tx *sql.Tx) error {
		qry := fmt.Sprintf("SELECT COUNT(*) FROM `%v`.`email_history` WHERE user_id = ? AND timestamp >= ? ", r.dbName)
		args := make([]interface{}, 0)
		args = append(args, r.c.UserUID(), time.Now().Add(-duration))
		if r.folderID != 0 {
			qry += "AND folder_id = ? "
			args = append(args, r.folderID)
		}
		if offset != 0 {
			if desc {
				qry += "AND `id` <= ?"
			} else {
				qry += "AND `id` >= ?"
			}
			args = append(args, offset)
		}

		if err := tx.QueryRow(qry, args...).Scan(&count); err != nil {
			return err
		}
		return nil
	}

	if err := r.queryer.Query(f); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *EmailStorage) GetLastEmailHistory(emailID uint64, lock database.LockMode) (history backend.EmailHistory, err error) {
	f := func(tx *sql.Tx) error {
		qry := "SELECT A.`id`, A.`operation`, A.`read`, A.`timestamp`, "
		qry += "B.`id`, B.`from`, B.`to`, B.`reply_to`, B.`cc`, B.`subject`, B.`body`, B.`charset`, B.`timestamp` "
		qry += "FROM `" + r.dbName + "`.`email_history` A INNER JOIN `" + r.dbName + "`.`email` B "
		qry += "ON A.email_id = B.id "
		qry += "WHERE B.id = ? AND A.user_id = ? "
		args := make([]interface{}, 0)
		args = append(args, emailID, r.c.UserUID())
		if r.folderID != 0 {
			qry += "AND A.folder_id = ? "
			args = append(args, r.folderID)
		}
		qry += "ORDER BY A.`id` DESC LIMIT 1"
		qry += mysql.GetLockCmd(lock)

		v := EmailHistory{}
		var replyTo, cc sql.NullString
		var from, to, eoe string
		if err := tx.QueryRow(qry, args...).Scan(&v.id, &eoe, &v.value.Seen, &v.timestamp,
			&v.value.ID, &from, &to, &replyTo, &cc, &v.value.Subject,
			&v.value.Body, &v.value.Charset, &v.value.Date); err != nil {
			return err
		}
		v.value.From = parseAddress(from)
		v.value.To = parseAddressList(to)
		if replyTo.Valid {
			v.value.ReplyTo = parseAddressList(replyTo.String)
		}
		if cc.Valid {
			v.value.Cc = parseAddressList(cc.String)
		}
		v.operation = ConvToBackendEmailOperation(eoe)

		// Get Attachment
		qry = "SELECT `id`, `email_id`, `content_type`, `content_id`, `name`, `size`, `method`, `order` "
		qry += "FROM `" + r.dbName + "`.`attachment` "
		qry += "WHERE `email_id` = ?"
		qry += mysql.GetLockCmd(lock)

		rows, err := tx.Query(qry, emailID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			a := new(Attachment)
			a.manager = r
			if err := rows.Scan(&a.id, &a.emailID, &a.contentType, &a.contentID,
				&a.name, &a.size, &a.method, &a.order); err != nil {
				return err
			}
			e, _ := v.Value()
			e.Attachments = append(e.Attachments, a)
		}
		history = &v

		return nil
	}

	if err := r.queryer.Query(f); err != nil {
		return nil, err
	}
	return history, nil
}

// offset is a history ID as a starting position. desc means descending order if it is true,
// otherwise ascending order. Zero offset means the last item if desc is true.
func (r *EmailStorage) GetEmailHistories(offset, limit uint64, desc bool, lock database.LockMode) (histories []backend.EmailHistory, err error) {
	f := func(tx *sql.Tx) error {
		qry := "SELECT A.`id`, A.`operation`, A.`read`, A.`timestamp`, "
		qry += "B.`id`, B.`from`, B.`to`, B.`reply_to`, B.`cc`, B.`subject`, B.`body`, B.`charset`, B.`timestamp` "
		qry += "FROM `" + r.dbName + "`.`email_history` A INNER JOIN `" + r.dbName + "`.`email` B "
		qry += "ON A.email_id = B.id "
		qry += "WHERE A.user_id = ? "
		args := make([]interface{}, 0)
		args = append(args, r.c.UserUID())
		if r.folderID != 0 {
			qry += "AND A.folder_id = ? "
			args = append(args, r.folderID)
		}
		if offset != 0 {
			if desc {
				qry += "AND A.`id` <= ? "
			} else {
				qry += "AND A.`id` >= ? "
			}
			args = append(args, offset)
		}
		if desc {
			qry += "ORDER BY A.`id` DESC "
		} else {
			qry += "ORDER BY A.`id` ASC "
		}
		if limit != 0 {
			qry += "LIMIT ?"
			args = append(args, limit)
		}
		qry += mysql.GetLockCmd(lock)

		rows, err := tx.Query(qry, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		var ids []string
		for rows.Next() {
			v := EmailHistory{}
			var replyTo, cc sql.NullString
			var from, to, eoe string
			if err := rows.Scan(&v.id, &eoe, &v.value.Seen, &v.timestamp,
				&v.value.ID, &from, &to, &replyTo, &cc, &v.value.Subject,
				&v.value.Body, &v.value.Charset, &v.value.Date); err != nil {
				return err
			}
			v.value.From = parseAddress(from)
			v.value.To = parseAddressList(to)
			if replyTo.Valid {
				v.value.ReplyTo = parseAddressList(replyTo.String)
			}
			if cc.Valid {
				v.value.Cc = parseAddressList(cc.String)
			}
			v.operation = ConvToBackendEmailOperation(eoe)
			histories = append(histories, &v)
			ids = append(ids, strconv.FormatUint(v.value.ID, 10))
		}
		if len(ids) == 0 {
			return nil
		}

		// Get Attachment
		qry = "SELECT `id`, `email_id`, `content_type`, `content_id`, `name`, `size`, `method`, `order` "
		qry += "FROM `" + r.dbName + "`.`attachment` "
		qry += "WHERE `email_id` IN (" + strings.Join(ids, ", ") + ")"
		qry += mysql.GetLockCmd(lock)

		rows2, err := tx.Query(qry)
		if err != nil {
			return err
		}
		defer rows2.Close()
		for rows2.Next() {
			a := new(Attachment)
			a.manager = r
			if err := rows2.Scan(&a.id, &a.emailID, &a.contentType, &a.contentID,
				&a.name, &a.size, &a.method, &a.order); err != nil {
				return err
			}

			for _, v := range histories {
				if e, _ := v.Value(); e.ID == a.emailID {
					e.Attachments = append(e.Attachments, a)
					break
				}
			}
		}
		return nil
	}

	if err := r.queryer.Query(f); err != nil {
		return nil, err
	}
	return histories, nil
}

func (r *EmailStorage) DeleteEmailHistory(historyID uint64) error {
	f := func(tx *sql.Tx) error {
		qry := "DELETE FROM `" + r.dbName + "`.`email_history` WHERE id = ? and user_id = ? "
		args := make([]interface{}, 0)
		args = append(args, historyID, r.c.UserUID())
		if r.folderID != 0 {
			qry += "AND folder_id = ? "
			args = append(args, r.folderID)
		}
		result, err := tx.Exec(qry, args...)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return database.ErrNotFound
		}

		return nil
	}

	if err := r.queryer.Query(f); err != nil {
		return err
	}

	return nil
}

func (r *EmailStorage) GetAttachment(attID uint64) (attach backend.Attachment, err error) {
	f := func(tx *sql.Tx) error {
		qry := "SELECT A.`id`, A.`email_id`, A.`content_type`, A.`content_id`, A.`name`, A.`size`, A.`method`, A.`order` "
		qry += "FROM `" + r.dbName + "`.`attachment` A "
		qry += "INNER JOIN `" + r.dbName + "`.`email` B "
		qry += "ON A.`email_id` = B.`id` "
		qry += "WHERE A.`id` = ? AND B.`user_id` = ? AND B.`available` = TRUE "
		args := make([]interface{}, 0)
		args = append(args, attID, r.c.UserUID())
		if r.folderID != 0 {
			qry += "AND folder_id = ? "
			args = append(args, r.folderID)
		}

		a := new(Attachment)
		a.manager = r
		if err := tx.QueryRow(qry, args...).Scan(&a.id, &a.emailID, &a.contentType, &a.contentID,
			&a.name, &a.size, &a.method, &a.order); err != nil {
			return err
		}
		attach = a
		return nil
	}

	if err := r.queryer.Query(f); err != nil {
		return nil, err
	}
	return attach, nil
}
