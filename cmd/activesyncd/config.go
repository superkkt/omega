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

package main

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/dlintw/goconf"
	"github.com/superkkt/logger"
)

var (
	defaultConfigFile = flag.String("config", fmt.Sprintf("/usr/local/etc/%v.conf", programName), "absolute path of the configuration file")
)

type Config struct {
	LogLevel logger.Level
	DB       DSN
	Port     uint16
	TLS      struct {
		CertFile string
		KeyFile  string
	}
	AllowHTTP bool
	SMTP      SMTP
	Auth      AuthAPI
}

type AuthAPI struct {
	Host     string
	Port     uint16
	Username string
	Password string
}

type SMTP struct {
	Host string
	Port uint16
}

type DSN struct {
	Host         string
	Port         uint16
	Username     string
	Password     string
	ActiveSyncDB string
	BackendDB    string
}

func (r *Config) parseLogLevel(l string) error {
	switch strings.ToUpper(l) {
	case "DEBUG":
		r.LogLevel = logger.LevelDebug
	case "INFO":
		r.LogLevel = logger.LevelInfo
	case "WARNING":
		r.LogLevel = logger.LevelWarning
	case "ERROR":
		r.LogLevel = logger.LevelError
	case "FATAL":
		r.LogLevel = logger.LevelFatal
	default:
		return fmt.Errorf("invalid log level: %v", l)
	}

	return nil
}

func (r *Config) Read(configFile string) error {
	if len(configFile) == 0 {
		configFile = *defaultConfigFile
	}

	c, err := goconf.ReadConfigFile(configFile)
	if err != nil {
		return err
	}
	if err := r.readDefaultSection(c); err != nil {
		return err
	}
	if err := r.readDatabaseSection(c); err != nil {
		return err
	}
	if err := r.readSMTPSection(c); err != nil {
		return err
	}

	return nil
}

func (r *Config) readDefaultSection(c *goconf.ConfigFile) error {
	var err error

	logLevel, err := c.GetString("default", "log_level")
	if err != nil || len(logLevel) == 0 {
		return errors.New("invalid default/log_level in the config file")
	}
	if err := r.parseLogLevel(logLevel); err != nil {
		return err
	}

	port, err := c.GetInt("default", "port")
	if err != nil || port <= 0 || port > 65535 {
		return errors.New("empty or invalid default/port value")
	}
	r.Port = uint16(port)

	r.TLS.CertFile, err = c.GetString("default", "cert_file")
	if err != nil || len(r.TLS.CertFile) == 0 {
		return errors.New("empty default/cert_file value")
	}
	if r.TLS.CertFile[0] != '/' {
		return errors.New("default/cert_file should be specified as an absolute path")
	}

	r.TLS.KeyFile, err = c.GetString("default", "key_file")
	if err != nil || len(r.TLS.KeyFile) == 0 {
		return errors.New("empty default/key_file value")
	}
	if r.TLS.KeyFile[0] != '/' {
		return errors.New("default/key_file should be specified as an absolute path")
	}

	r.AllowHTTP, err = c.GetBool("default", "allow_http")
	if err != nil {
		return errors.New("invalid default/allow_http value")
	}

	return nil
}

func (r *Config) readDatabaseSection(c *goconf.ConfigFile) error {
	var err error

	r.DB.Host, err = c.GetString("database", "host")
	if err != nil || len(r.DB.Host) == 0 {
		return errors.New("empty database/host value")
	}

	port, err := c.GetInt("database", "port")
	if err != nil || port <= 0 || port > 65535 {
		return errors.New("empty or invalid database/port value")
	}
	r.DB.Port = uint16(port)

	r.DB.Username, err = c.GetString("database", "username")
	if err != nil || len(r.DB.Username) == 0 {
		return errors.New("empty database/username value")
	}

	r.DB.Password, err = c.GetString("database", "password")
	if err != nil || len(r.DB.Password) == 0 {
		return errors.New("empty database/password value")
	}

	r.DB.ActiveSyncDB, err = c.GetString("database", "activesync_db")
	if err != nil || len(r.DB.ActiveSyncDB) == 0 {
		return errors.New("empty database/activesync_db value")
	}

	r.DB.BackendDB, err = c.GetString("database", "backend_db")
	if err != nil || len(r.DB.BackendDB) == 0 {
		return errors.New("empty database/backend_db value")
	}

	return nil
}

func (r *Config) readSMTPSection(c *goconf.ConfigFile) error {
	var err error

	r.SMTP.Host, err = c.GetString("smtp", "host")
	if err != nil || len(r.SMTP.Host) == 0 {
		return errors.New("empty smtp/host value")
	}

	port, err := c.GetInt("smtp", "port")
	if err != nil || port <= 0 || port > 65535 {
		return errors.New("empty or invalid smtp/port value")
	}
	r.SMTP.Port = uint16(port)

	return nil
}
