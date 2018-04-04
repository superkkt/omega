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
	"flag"
	"fmt"
	"log/syslog"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/superkkt/omega/activesync"
	"github.com/superkkt/omega/activesync/eas25"
	"github.com/superkkt/omega/cert"
	"github.com/superkkt/omega/database/mysql"
	"github.com/superkkt/omega/database/mysql/backend"
	"github.com/superkkt/omega/database/mysql/eas"
	"github.com/superkkt/omega/mockup/authenticator"
	"github.com/superkkt/omega/smtp"

	"github.com/pkg/profile"
	"github.com/superkkt/logger"
	"golang.org/x/net/context"
)

const (
	programVersion = "0.2.0"
	programName    = "activesyncd"
)

var (
	showVersion = flag.Bool("version", false, "show program version and exit")
	profileMode = flag.String("profile.mode", "", "enable profiling mode, one of [cpu, mem, block]")
	profiler    interface {
		Stop()
	}
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	flag.Parse()

	if *showVersion {
		fmt.Printf("%v (Version: %v)\n", programName, programVersion)
		os.Exit(0)
	}
	if *profileMode != "" {
		switch strings.ToUpper(*profileMode) {
		case "CPU":
			profiler = profile.Start(profile.CPUProfile)
		case "MEM":
			profiler = profile.Start(profile.MemProfile)
		case "BLOCK":
			profiler = profile.Start(profile.BlockProfile)
		default:
			logger.Fatal("profile.mode should be one of [cpu, mem, block]")
		}
	}

	config := new(Config)
	if err := config.Read(*defaultConfigFile); err != nil {
		logger.Fatal(fmt.Sprintf("Failed to read configurations: %v", err))
	}
	db, err := initDatabase(config)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to init database: %v", err))
	}
	cert, err := cert.NewLoader(config.TLS.CertFile, config.TLS.KeyFile)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to load the certification: %v", err))
	}
	// TODO: Implement a real authenticator.
	auth := &authenticator.MockAuth{Username: "test", Password: "test"}
	ctx, cancel := context.WithCancel(context.Background())
	go signalHandler(cancel)

	initSyslog(config)
	logger.Info(fmt.Sprintf("%v is initialized..", programName))
	asConfig := activesync.Config{
		Port:      config.Port,
		Cert:      cert,
		AllowHTTP: config.AllowHTTP,
		Param: activesync.Parameter{
			Authenticator:  auth,
			ASStorage:      eas.New(config.DB.ActiveSyncDB),
			BackendStorage: backend.New(config.DB.BackendDB),
			Transaction:    db,
			Mailer:         smtp.New(config.SMTP.Host, config.SMTP.Port),
		},
	}
	// ActiveSync Protocol Version 2.5
	activesync.RegisterFactory(eas25.NewFactory())
	as := activesync.NewListener(asConfig)
	if err := as.Run(ctx); err != nil {
		logger.Fatal(fmt.Sprintf("Failed to run the listener: %v", err))
	}
	logger.Info(fmt.Sprintf("%v is finished..", programName))
}

func signalHandler(shutdown context.CancelFunc) {
	c := make(chan os.Signal, 5)
	// Following signals will be transferred to the channel c.
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGPIPE)

	for {
		switch <-c {
		case syscall.SIGTERM, syscall.SIGINT:
			logger.Info("Shutting down...")
			shutdown()
			// Timeout for cancelation
			time.Sleep(3 * time.Second)
			if *profileMode != "" {
				profiler.Stop()
			}
			os.Exit(0)
		default:
			logger.Warning(fmt.Sprintf("Received %v signal!", c))
		}
	}
}

func initSyslog(conf *Config) {
	log, err := syslog.NewLogger(syslog.LOG_ERR|syslog.LOG_DAEMON, 0)
	if err != nil {
		log.Fatalf("Failed to init syslog: %v\n", err)
	}
	logger.SetLogger(log)
	logger.SetLogLevel(conf.LogLevel)
	logger.SetPrefix(func() string {
		return fmt.Sprintf("TID=%v, ", getGoRoutineID())
	})
}

func getGoRoutineID() string {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	return strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
}

func initDatabase(config *Config) (db *mysql.MySQL, err error) {
	// NOTE: Enable clientFoundRows to cause an UPDATE to return the number of matching rows instead of the number of rows changed.
	return mysql.NewMySQL(config.DB.Host, config.DB.Username, config.DB.Password, config.DB.Port, true)
}
