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
	"crypto/tls"
	"fmt"
	"net/http"
	"runtime"

	"github.com/superkkt/omega/backend"

	"github.com/superkkt/logger"
	"golang.org/x/net/context"
)

type Listener struct {
	config Config
}

type Config struct {
	Port      uint16
	Cert      CertLoader
	AllowHTTP bool
	Param     Parameter
}

type CertLoader interface {
	GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error)
}

func NewListener(conf Config) *Listener {
	return &Listener{
		config: conf,
	}
}

// TODO: Use ctx for graceful shutdown, but how can we use it with http.ListenAndServeTLS?
func (r *Listener) Run(ctx context.Context) error {
	if len(factories) == 0 {
		panic("empty factories")
	}

	http.HandleFunc("/Microsoft-Server-ActiveSync", r.dispatcher)
	// Allow non-secured HTTP connection for debugging purpose only
	if r.config.AllowHTTP {
		go func() {
			if err := http.ListenAndServe(":http", nil); err != nil {
				logger.Error(fmt.Sprintf("Failed to listen on HTTP: %v", err))
			}
		}()
	}
	// TLS listener
	srv := &http.Server{
		Addr: fmt.Sprintf(":%v", r.config.Port),
		TLSConfig: &tls.Config{
			GetCertificate: r.config.Cert.GetCertificate,
		},
	}
	return srv.ListenAndServeTLS("", "")
}

func (r *Listener) auth(req *http.Request) (backend.Credential, error) {
	username, password, ok := req.BasicAuth()
	if !ok {
		return unauthorized{}, nil
	}

	return r.config.Param.Authenticator.Auth(username, password)
}

type unauthorized struct{}

func (r unauthorized) IsAuthorized() bool {
	return false
}

func (r unauthorized) UserID() string {
	return ""
}

func (r unauthorized) UserUID() uint64 {
	return 0
}

func (r *Listener) dispatcher(w http.ResponseWriter, req *http.Request) {
	logger.Debug(fmt.Sprintf("Total number of goroutines = %v", runtime.NumGoroutine()))
	logger.Debug(fmt.Sprintf("Client: %v, Method: %v, URL: %v, Header: %v", req.RemoteAddr, req.Method, req.URL, removeAuthInfo(req.Header)))

	c, err := r.auth(req)
	if err != nil {
		logger.Error(fmt.Sprintf("activesync: failed to authorize a new request: %v", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if c.IsAuthorized() == false {
		logger.Info(fmt.Sprintf("Unauthorized: username=%v", c.UserID()))
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Indicates that all or part of the response message is intended for a single
	// user and MUST NOT be cached by a shared cache, such as a proxy server.
	w.Header()["Cache-Control"] = []string{"private"}

	switch req.Method {
	case "OPTIONS":
		sendOptionsResponse(w)
	case "POST":
		r.dispatch(c, w, req)
	default:
		w.Header()["Allow"] = []string{"OPTIONS,POST"}
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Only allows OPTIONS and POST HTTP methods"))
	}
}

// removeAuthInfo returns a deep copy of h except the Authorization header field.
func removeAuthInfo(h http.Header) http.Header {
	h2 := make(http.Header, len(h))
	for k, vv := range h {
		// Remove the Authorization header field to hide user's password.
		if k == "Authorization" {
			continue
		}
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		h2[k] = vv2
	}
	return h2
}

func sendOptionsResponse(w http.ResponseWriter) {
	w.Header()["Allow"] = []string{"OPTIONS,POST"}
	w.Header()["MS-ASProtocolVersions"] = []string{factories.Versions()}
	w.Header()["MS-ASProtocolCommands"] = []string{factories.Commands()}
	w.WriteHeader(http.StatusOK)
}

func (r *Listener) dispatch(c backend.Credential, w http.ResponseWriter, req *http.Request) {
	// Check DeviceId
	if req.URL.Query().Get("DeviceId") == "" {
		logger.Debug(fmt.Sprintf("Missing DeviceId URI parameter from %v", req.RemoteAddr))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Missing DeviceId URI parameter"))
		return
	}

	// Check Protocol Version
	version := req.Header.Get("MS-ASProtocolVersion")
	if version == "" {
		logger.Debug(fmt.Sprintf("Missing MS-ASProtocolVersion header from %v", req.RemoteAddr))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Missing MS-ASProtocolVersion header"))
		return
	}

	// Generate a handler for the version that the client requests
	f := factories[version]
	if f == nil {
		logger.Debug(fmt.Sprintf("Unsupported ActiveSync protocol version from %v", req.RemoteAddr))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Unsupported ActiveSync protocol version"))
		return
	}
	h := f.New(r.config.Param)
	// Process the request
	h.Handle(c, w, req)
}
