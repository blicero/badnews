// /home/krylon/go/src/github.com/blicero/badnews/main.go
// -*- mode: go; coding: utf-8; -*-
// Created on 18. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-10-30 17:52:52 krylon>

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/blicero/badnews/common"
	"github.com/blicero/badnews/common/path"
	"github.com/blicero/badnews/reader"
	"github.com/blicero/badnews/web"
)

func main() {
	fmt.Printf("%s %s built on %s\n",
		common.AppName,
		common.Version,
		common.BuildStamp.Format(common.TimestampFormat))

	var (
		err     error
		rdr     *reader.Reader
		srv     *web.Server
		sigq    chan os.Signal
		minlog  = "TRACE"
		baseDir = common.Path(path.Base)
		addr    = fmt.Sprintf("[::1]:%d", common.Port)
	)

	flag.StringVar(&baseDir, "basedir", baseDir, "Path for application-specific files")
	flag.StringVar(&addr, "addr", addr, "Address for the web server to listen on")
	flag.StringVar(&minlog, "loglevel", minlog, "Minimum level for log messages to be logged")
	flag.Parse()

	if baseDir != common.Path(path.Base) {
		if err = common.SetBaseDir(baseDir); err != nil {
			fmt.Fprintf(
				os.Stderr,
				"Failed to set Base Directory to %s: %s\n",
				baseDir,
				err.Error())
			os.Exit(1)
		}
	} else if err = common.InitApp(); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Error initializing application environment: %s\n",
			err.Error())
		os.Exit(2)
	}

	if rdr, err = reader.New(4); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Error creating Reader: %s\n",
			err.Error())
		os.Exit(2)
	} else if srv, err = web.Create(addr); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Error creating Web server: %s\n",
			err.Error())
		os.Exit(2)
	}

	rdr.Start()
	go srv.ListenAndServe()

	sigq = make(chan os.Signal, 2)

	signal.Notify(sigq, os.Interrupt, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM)

	for {
		sig := <-sigq

		fmt.Fprintf(
			os.Stderr,
			"Received signal %s, quitting.\n",
			sig)
		os.Exit(0)
	}
}
