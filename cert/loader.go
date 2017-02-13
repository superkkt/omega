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

package cert

import (
	"crypto/tls"
	"fmt"

	"github.com/superkkt/logger"
)

type Loader struct {
	certFile, keyFile string
	cached            tls.Certificate
}

func NewLoader(certFile, keyFile string) (*Loader, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	return &Loader{
		certFile: certFile,
		keyFile:  keyFile,
		cached:   cert,
	}, nil
}

func (r *Loader) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(r.certFile, r.keyFile)
	if err != nil {
		logger.Error(fmt.Sprintf("cert: failed to read new certifications: %v", err))
		logger.Warning("cert: fallback to the cached certification")
		// Fallback
		return &r.cached, nil
	}
	r.cached = cert

	return &cert, nil
}
