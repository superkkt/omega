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
	"bytes"
	"net/mail"
	"regexp"

	"github.com/jhillyerd/go.enmime"
)

type mimeMsg struct {
	// Raw MIME message
	raw []byte
	// Normalized MIME message
	norm []byte
	// Recipient addresses
	rcpts  []string
	parsed struct {
		header mail.Header
		mime   *enmime.MIMEBody
	}
}

func newMIMEMsg(raw []byte) (*mimeMsg, error) {
	m, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	// Parse message body with enmime
	mime, err := enmime.ParseMIMEBody(m)
	if err != nil {
		return nil, err
	}
	addrs, err := getRcptAddrs(mime)
	if err != nil {
		return nil, err
	}

	return &mimeMsg{
		raw:   raw,
		norm:  removeBCC(convertToCRLF(raw)),
		rcpts: addrs,
		parsed: struct {
			header mail.Header
			mime   *enmime.MIMEBody
		}{
			header: m.Header,
			mime:   mime,
		},
	}, nil
}

func convertToCRLF(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	tokens := bytes.Split(data, []byte("\n"))
	out := make([][]byte, len(tokens))
	for i, v := range tokens {
		if len(v) == 0 {
			continue
		}
		switch v[len(v)-1] {
		case '\r':
			out[i] = v[:len(v)-1]
		default:
			out[i] = v
		}
	}

	return bytes.Join(out, []byte("\r\n"))
}

func removeBCC(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	regexp := regexp.MustCompile("(?im)^BCC:.*\\r\\n")
	return regexp.ReplaceAll(data, []byte(""))
}

func getRcptAddrs(mime *enmime.MIMEBody) (addrs []string, err error) {
	to, err := mime.AddressList("To")
	// To is required.
	if err != nil {
		return nil, err
	}
	addrs = append(addrs, convertToPlainAddrs(to)...)

	cc, err := mime.AddressList("Cc")
	// Cc is optional.
	if err != nil && err != mail.ErrHeaderNotPresent {
		return nil, err
	}
	if len(cc) > 0 {
		addrs = append(addrs, convertToPlainAddrs(cc)...)
	}

	bcc, err := mime.AddressList("Bcc")
	// Bcc is optional.
	if err != nil && err != mail.ErrHeaderNotPresent {
		return nil, err
	}
	if len(bcc) > 0 {
		addrs = append(addrs, convertToPlainAddrs(bcc)...)
	}

	return addrs, nil
}

func convertToPlainAddrs(addrs []*mail.Address) []string {
	out := make([]string, len(addrs))
	for i, v := range addrs {
		out[i] = v.Address
	}

	return out
}

func addLineBreak(data string) string {
	if len(data) == 0 {
		return ""
	}

	buf := bytes.NewBufferString(data)
	line := make([]byte, 76)
	var out bytes.Buffer
	for {
		n, err := buf.Read(line)
		if err != nil {
			break
		}
		out.Write(line[:n])
		out.WriteString("\r\n")
	}

	return out.String()
}
