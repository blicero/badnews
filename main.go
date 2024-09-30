// /home/krylon/go/src/github.com/blicero/badnews/main.go
// -*- mode: go; coding: utf-8; -*-
// Created on 18. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-30 13:55:06 krylon>

package main

import (
	"fmt"
	"os"

	"github.com/blicero/badnews/common"
	"github.com/blicero/badnews/reader"
	"github.com/blicero/badnews/web"
)

func main() {
	fmt.Printf("%s %s built on %s\n",
		common.AppName,
		common.Version,
		common.BuildStamp.Format(common.TimestampFormat))
	fmt.Println("IMPLEMENT ME!")

	var (
		err  error
		rdr  *reader.Reader
		srv  *web.Server
		addr = fmt.Sprintf("[::1]:%d", common.Port)
	)

	if err = common.InitApp(); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Error initializing application environment: %s\n",
			err.Error())
		os.Exit(2)
	} else if rdr, err = reader.New(4); err != nil {
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
	srv.ListenAndServe()
}
