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

package smtp

import (
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"regexp"
	"time"
)

const (
	timeout = 30 * time.Second
)

type Sendmail struct {
	host string
	port uint16
}

// TODO: Add authentication
func New(host string, port uint16) *Sendmail {
	return &Sendmail{
		host: host,
		port: port,
	}
}

func validateEmail(email string) bool {
	Re := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)
	return Re.MatchString(email)
}

func (r *Sendmail) Send(from string, to []string, msg []byte) error {
	if !validateEmail(from) {
		return fmt.Errorf("invalid from address: %v", from)
	}
	if len(to) == 0 {
		return errors.New("empty recipient address")
	}
	for _, v := range to {
		if !validateEmail(v) {
			return fmt.Errorf("invalid to address: %v", v)
		}
	}
	if len(msg) == 0 {
		return errors.New("empty msg body")
	}

	d := net.Dialer{Timeout: timeout}
	conn, err := d.Dial("tcp", fmt.Sprintf("%v:%v", r.host, r.port))
	if err != nil {
		return err
	}
	conn.SetDeadline(time.Now().Add(timeout))

	c, err := smtp.NewClient(conn, r.host)
	if err != nil {
		return err
	}
	defer c.Close()
	// Set the sender address
	if err := c.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		// Set the recipient address
		if err := c.Rcpt(rcpt); err != nil {
			return err
		}
	}
	// Set the email body.
	wc, err := c.Data()
	if err != nil {
		return err
	}
	if _, err = wc.Write(msg); err != nil {
		return err
	}
	if err = wc.Close(); err != nil {
		return err
	}

	// Send the QUIT command and close the connection.
	return c.Quit()
}
