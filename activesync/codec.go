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

package activesync

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	wbxml "github.com/superkkt/go-libwbxml"
	"github.com/superkkt/logger"
)

// XXX: debugging
var log *os.File

func init() {
	logFile := "/tmp/omega_log"
	fd, err := os.OpenFile(logFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to open a debugging log file: %v", err))
	}
	log = fd
	logger.Info(fmt.Sprintf("Logging WBXML read/write packets into %v", logFile))
}

func ParseWBXMLRequest(req *http.Request, dest interface{}) error {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %v", err)
	}
	if len(body) == 0 {
		logger.Debug("Empty WBXML request body")
		return nil
	}

	bodyDec, err := wbxml.Decode(body)
	if err != nil {
		return fmt.Errorf("failed to decode WBXML: %v", err)
	}
	// XXX: debugging
	if log != nil {
		if _, err := log.Write([]byte(fmt.Sprintf("C: %v: %v: %v\n", req.RemoteAddr, time.Now(), bodyDec))); err != nil {
			logger.Error(fmt.Sprintf("Failed to write debugging log: %v", err))
		}
	}

	return xml.Unmarshal([]byte(bodyDec), dest)
}

func writeWBXMLResponse(w http.ResponseWriter, xml string) error {
	if !strings.HasPrefix(xml, "<?xml") {
		header := `<?xml version="1.0" encoding="utf-8"?>`
		docType := `<!DOCTYPE ActiveSync PUBLIC "-//MICROSOFT//DTD ActiveSync//EN" "http://www.microsoft.com/">`
		xml = header + docType + xml
	}

	encoded, err := wbxml.Encode(xml)
	if err != nil {
		return err
	}

	// TODO: Remove this debugging codes
	dec, err := wbxml.Decode(encoded)
	if err != nil {
		return fmt.Errorf("failed to self-decoding: %v", err)
	}
	// XXX: debugging
	if log != nil {
		if _, err := log.Write([]byte(fmt.Sprintf("S: %v: %v\n", time.Now(), dec))); err != nil {
			logger.Error(fmt.Sprintf("Failed to write debugging log: %v", err))
		}
	}

	w.Header().Set("Content-Type", "application/vnd.ms-sync.wbxml")
	if _, err := w.Write(encoded); err != nil {
		return err
	}

	return nil
}

// ResponseWriter is a buffered response writer.
type ResponseWriter struct {
	http.ResponseWriter
	status int
	buf    bytes.Buffer
	wbxml  bool // Need WBXML encoding?
}

func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{
		ResponseWriter: w,
	}
}

func (r *ResponseWriter) WriteHeader(status int) {
	r.status = status
}

func (r *ResponseWriter) Write(v []byte) {
	// Buffer.Write's error is always nil.
	r.buf.Write(v)
}

func (r *ResponseWriter) Clear() {
	r.status = 0
	r.buf.Reset()
}

func (r *ResponseWriter) Flush() error {
	if r.status != 0 {
		r.WriteHeader(r.status)
	}
	if r.buf.Len() == 0 {
		return nil
	}
	if !r.wbxml {
		_, err := r.buf.WriteTo(r.ResponseWriter)
		return err
	}

	return writeWBXMLResponse(r.ResponseWriter, r.buf.String())
}

func (r *ResponseWriter) SetWBXML(v bool) {
	r.wbxml = v
}
