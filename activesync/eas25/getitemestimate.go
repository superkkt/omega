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
	"net/http"

	"github.com/Muzikatoshi/omega/activesync"
	"github.com/Muzikatoshi/omega/database"

	"github.com/superkkt/logger"
)

func (r *handler) handleGetItemEstimate(tx database.Transaction) error {
	// GetItemEstimate response is given in WBXML encoding.
	r.resp.SetWBXML(true)

	reqBody := struct {
		XMLName     xml.Name `xml:"GetItemEstimate"`
		Collections struct {
			Collection []struct {
				CollectionId uint64
			}
		}
	}{}

	if err := activesync.ParseWBXMLRequest(r.req, &reqBody); err != nil {
		r.badRequest = true
		return fmt.Errorf("ParseWBXMLRequest: %v", err)
	}
	logger.Debug(fmt.Sprintf("GetItemEstimate request: %+v", reqBody))

	// TODO: Implement this function
	r.resp.WriteHeader(http.StatusNotImplemented)

	return nil
}
