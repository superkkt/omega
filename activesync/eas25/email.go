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
	"strings"

	"github.com/Muzikatoshi/omega/backend"
	"github.com/Muzikatoshi/omega/database"
)

type Email struct {
	To           string
	Cc           string
	From         string
	Subject      string
	DateReceived string
	DisplayTo    string
	ThreadTopic  string
	Importance   string
	Read         string
	Attachments  struct {
		Attachment []Attach
	}
	MIMETruncated string
	MIMESize      uint64
	MIMEData      string
	BodyTruncated string
	BodySize      uint64
	Body          string
	MessageClass  string
	InternetCPID  string
}

type Attach struct {
	AttMethod   string
	AttSize     uint64
	DisplayName string
	AttName     string
}

type email struct {
	*backend.Email
	options SyncOptions
	manager backend.EmailManager
}

func (r *email) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	switch r.options.MIMESupport {
	case "0", "1":
		return r.marshalRegular(e, start)
	default: // Assume 2 that means Send MIME data for all messages.
		return r.marshalMIME(e, start)
	}
}

func encodeElem(e *xml.Encoder, name string, value interface{}) error {
	// Ignore empty string element
	if v, ok := value.(string); ok {
		if len(v) == 0 {
			return nil
		}
	}

	return e.EncodeElement(value, xml.StartElement{Name: xml.Name{Local: name}})
}

func (r *email) marshalRegular(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(xml.StartElement{Name: xml.Name{Local: "ApplicationData"}}); err != nil {
		return err
	}
	if err := r.marshalBasic(e); err != nil {
		return err
	}

	body, truncated := truncateBody(r.Body, r.options.Truncation)
	if len(body) == 0 {
		// Add a space to avoid empty body element that causes a protocol error (see #8114).
		body = " "
	}
	if err := encodeElem(e, "email:Body", body); err != nil {
		return err
	}
	if truncated {
		if err := encodeElem(e, "email:BodyTruncated", "1"); err != nil {
			return err
		}
		// BodySize means original body size in characters.
		if err := encodeElem(e, "email:BodySize", len([]rune(r.Body))); err != nil {
			return err
		}
	} else {
		if err := encodeElem(e, "email:BodyTruncated", "0"); err != nil {
			return err
		}
	}

	if err := r.marshalAttachments(e); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "ApplicationData"}})
}

func (r *email) marshalBasic(e *xml.Encoder) error {
	if err := encodeElem(e, "email:To", getAddrStr(r.To, ",")); err != nil {
		return err
	}
	if err := encodeElem(e, "email:Cc", getAddrStr(r.Cc, ",")); err != nil {
		return err
	}
	if err := encodeElem(e, "email:From", getAddrStr([]backend.EmailAddress{r.From}, ",")); err != nil {
		return err
	}
	if err := encodeElem(e, "email:ReplyTo", getAddrStr(r.ReplyTo, ";")); err != nil {
		return err
	}
	if err := encodeElem(e, "email:Subject", r.Subject); err != nil {
		return err
	}
	// DateReceived should be given in UTC.
	if err := encodeElem(e, "email:DateReceived", r.Date.UTC().Format("2006-01-02T15:04:05.000Z")); err != nil {
		return err
	}
	if err := encodeElem(e, "email:DisplayTo", getNameStr(r.To)); err != nil {
		return err
	}
	if err := encodeElem(e, "email:ThreadTopic", r.Subject); err != nil {
		return err
	}
	// TODO: Set Importance dynamically
	if err := encodeElem(e, "email:Importance", "1"); err != nil {
		return err
	}

	seen := "0"
	if r.Seen {
		seen = "1"
	}
	return encodeElem(e, "email:Read", seen)
}

// https://msdn.microsoft.com/en-us/library/windows/desktop/dd317756(v=vs.85).aspx
func internetCPID(charset string) string {
	norm, ok := normCharset(charset)
	if !ok {
		// We don't know the charset, so fallback to EUC-KR.
		return "51949"
	}

	switch norm {
	case "utf-8":
		return "65001"
	case "utf-16le": // UTF-16 Litten Endian
		return "1200"
	case "utf-16be": // UTF-16 Big Endian
		return "1201"
	case "gbk": // simplifiedchinese.GBK
		return "936"
	case "gb18030": // simplifiedchinese.GB18030
		return "54936"
	case "hz-gb-2312": // simplifiedchinese.HZGB2312
		return "52936"
	case "big5": // traditionalchinese.Big5
		return "950"
	case "euc-jp": // japanese.EUCJP
		return "20932"
	case "iso-2022-jp": // japanese.ISO2022JP
		return "50222"
	case "shift_jis": // japanese.ShiftJIS
		return "932"
	default:
		// Fallback to EUC-KR
		return "51949"
	}
}

func (r *email) marshalAttachments(e *xml.Encoder) error {
	if len(r.Attachments) == 0 {
		return nil
	}

	if err := e.EncodeToken(xml.StartElement{Name: xml.Name{Local: "email:Attachments"}}); err != nil {
		return err
	}
	for _, v := range r.Attachments {
		if err := e.EncodeToken(xml.StartElement{Name: xml.Name{Local: "email:Attachment"}}); err != nil {
			return err
		}

		attMethod := "1" // Normal attachment
		if strings.ToLower(v.ContentType()) == "message/rfc822" {
			attMethod = "5" // Embedded message (EML)
		}
		if err := encodeElem(e, "email:AttMethod", attMethod); err != nil {
			return err
		}
		if err := encodeElem(e, "email:AttSize", v.Size()); err != nil {
			return err
		}
		if err := encodeElem(e, "email:DisplayName", v.Name()); err != nil {
			return err
		}
		// Attachement name consists of the folderID and the attachementID.
		attName := fmt.Sprintf("%v:%v", r.manager.FolderID(), v.ID())
		if err := encodeElem(e, "email:AttName", attName); err != nil {
			return err
		}
		if err := e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "email:Attachment"}}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "email:Attachments"}})
}

func (r *email) marshalMIME(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(xml.StartElement{Name: xml.Name{Local: "ApplicationData"}}); err != nil {
		return err
	}
	if err := r.marshalBasic(e); err != nil {
		return err
	}
	if err := r.marshalAttachments(e); err != nil {
		return err
	}

	raw, err := r.manager.GetRawEmail(r.ID, database.LockNone)
	if err != nil {
		return err
	}
	mime, truncated := truncateMIME(string(raw), r.options.MIMETruncation)
	if len(mime) == 0 {
		// Add a space to avoid empty MIMEData element that causes a protocol error (see #8114).
		mime = " "
	}
	if truncated {
		if err := encodeElem(e, "email:MIMETruncated", "1"); err != nil {
			return err
		}
		if err := encodeElem(e, "email:MIMESize", len(raw)); err != nil {
			return err
		}
	} else {
		if err := encodeElem(e, "email:MIMETruncated", "0"); err != nil {
			return err
		}
	}
	if err := encodeElem(e, "email:MIMEData", mime); err != nil {
		return err
	}

	// Normal e-mail message
	if err := encodeElem(e, "email:MessageClass", "IPM.Note"); err != nil {
		return err
	}
	if err := encodeElem(e, "email:InternetCPID", internetCPID(r.Charset)); err != nil {
		return err
	}

	return e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "ApplicationData"}})
}

func truncateBody(body, truncation string) (result string, truncated bool) {
	switch truncation {
	case "0": // Truncate all body text
		return truncateInChars(body, 0)
	case "1": // Truncate body text that is more than 512 characters
		return truncateInChars(body, 512)
	case "2": // Truncate body text that is more than 1,024 characters
		return truncateInChars(body, 1024)
	case "3": // Truncate body text that is more than 2,048 characters
		return truncateInChars(body, 2048)
	case "4": // Truncate body text that is more than 5,120 characters
		return truncateInChars(body, 5120)
	case "5": // Truncate body text that is more than 10,240 characters
		return truncateInChars(body, 10240)
	case "6": // Truncate body text that is more than 20,480 characters
		return truncateInChars(body, 20480)
	case "7": // Truncate body text that is more than 51,200 characters
		return truncateInChars(body, 51200)
	case "8": // Truncate body text that is more than 102,400 characters
		return truncateInChars(body, 102400)
	default: // No truncation
		return body, false
	}
}

// charLen is the character length in UTF-8.
func truncateInChars(str string, charLen int) (result string, truncated bool) {
	if str == "" {
		return "", false
	}
	if charLen == 0 {
		return "", true
	}

	r := []rune(str)
	if len(r) <= charLen {
		return str, false
	}

	return string(r[:charLen]), true
}

func truncateMIME(mime, truncation string) (result string, truncated bool) {
	switch truncation {
	case "0": // Truncate all body text
		return truncateInBytes(mime, 0)
	case "1": // Truncate text over 4,096 characters.
		return truncateInBytes(mime, 4096)
	case "2": // Truncate text over 5,120 characters.
		return truncateInBytes(mime, 5120)
	case "3": // Truncate text over 7,168 characters.
		return truncateInBytes(mime, 7168)
	case "4": // Truncate text over 10,240 characters.
		return truncateInBytes(mime, 10240)
	case "5": // Truncate text over 20,480 characters.
		return truncateInBytes(mime, 20480)
	case "6": // Truncate text over 51,200 characters.
		return truncateInBytes(mime, 51200)
	case "7": // Truncate text over 102,400 characters.
		return truncateInBytes(mime, 102400)
	default: // No truncation
		return mime, false
	}
}

// length is the string length in bytes.
func truncateInBytes(str string, length int) (result string, truncated bool) {
	if str == "" {
		return "", false
	}
	if length == 0 {
		return "", true
	}

	if len(str) <= length {
		return str, false
	}

	return str[:length], true
}

func getNameStr(addr []backend.EmailAddress) string {
	output := ""
	for i, v := range addr {
		if i > 0 {
			output += "; "
		}

		if v.Name == "" {
			output += v.Address
		} else {
			output += v.Name
		}
	}

	return output
}

func getAddrStr(addr []backend.EmailAddress, sep string) string {
	output := ""
	for i, v := range addr {
		if i > 0 {
			output += sep + " "
		}

		var t string
		if v.Name == "" {
			t = v.Address
		} else {
			t = fmt.Sprintf(`"%v" <%v>`, v.Name, v.Address)
		}
		// 32,768 is the maximum length of To, Cc, From, and ReplyTo addresses.
		if len(output)+len(t) > 32768 {
			break
		}
		output += t
	}

	return output
}
