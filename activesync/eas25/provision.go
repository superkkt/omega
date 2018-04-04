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

package eas25

import (
	"encoding/xml"
	"fmt"

	"github.com/superkkt/omega/activesync"

	"github.com/superkkt/logger"
)

// Initial provision response that does not require any security policy
var initResponse = `<Provision xmlns="Provision:"><Policies><Policy><PolicyType>MS-WAP-Provisioning-XML</PolicyType><Status>1</Status><Data>&lt;wap-provisioningdoc&gt;&lt;characteristic type="SecurityPolicy"&gt;&lt;parm name="4131" value="1"/&gt;&lt;/characteristic&gt;&lt;characteristic type="Registry"&gt;&lt;characteristic type="HKLM\Comm\Security\Policy\LASSD\AE\{50C13377-C66D-400C-889E-C316FC4AB374}"&gt;&lt;parm name="AEFrequencyType" value="0"/&gt;&lt;parm name="AEFrequencyValue" value="0"/&gt;&lt;/characteristic&gt;&lt;characteristic type="HKLM\Comm\Security\Policy\LASSD"&gt;&lt;parm name="DeviceWipeThreshold" value="16"/&gt;&lt;parm name="CodewordFrequency" value="-1"/&gt;&lt;/characteristic&gt;&lt;characteristic type="HKLM\Comm\Security\Policy\LASSD\LAP\lap_pw"&gt;&lt;parm name="MinimumPasswordLength" value="1"/&gt;&lt;parm name="PasswordComplexity" value="2"/&gt;&lt;/characteristic&gt;&lt;/characteristic&gt;&lt;/wap-provisioningdoc&gt;</Data><PolicyKey>1</PolicyKey></Policy></Policies></Provision>`

// Second provision response that always succeed
var secondResponse = `<Provision xmlns="Provision:"><Status>1</Status><Policies><Policy><PolicyType>MS-WAP-Provisioning-XML</PolicyType><Status>1</Status><PolicyKey>2</PolicyKey></Policy></Policies></Provision>`

func (r *handler) handleProvision() error {
	// Provision response is given in WBXML encoding.
	r.resp.SetWBXML(true)

	reqBody := struct {
		XMLName  xml.Name `xml:"Provision"`
		Policies struct {
			Policy struct {
				PolicyType string
				PolicyKey  string
				Status     int
			}
		}
	}{}

	if err := activesync.ParseWBXMLRequest(r.req, &reqBody); err != nil {
		r.badRequest = true
		return fmt.Errorf("ParseWBXMLRequest: %v", err)
	}
	logger.Debug(fmt.Sprintf("Provision request: %+v", reqBody))

	// Validation
	if reqBody.Policies.Policy.PolicyType != "MS-WAP-Provisioning-XML" {
		r.badRequest = true
		return fmt.Errorf("invalid Provision PolicyType: %v", reqBody.Policies.Policy.PolicyType)
	}

	var xml string
	// Initial request?
	if reqBody.Policies.Policy.PolicyKey == "" {
		xml = initResponse
	} else {
		xml = secondResponse
	}
	r.resp.Write([]byte(xml))

	return nil
}
