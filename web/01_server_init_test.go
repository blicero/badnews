// /home/krylon/go/src/github.com/blicero/scrollmaster/server/01_server_init_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 25. 08. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-28 15:52:37 krylon>

package web

import (
	"fmt"
	"testing"
	"time"
)

func TestServerCreate(t *testing.T) {
	var err error

	addr = fmt.Sprintf("[::1]:%d", testPort)

	if srv, err = Create(addr); err != nil {
		srv = nil
		t.Fatalf("Error creating Server: %s",
			err.Error())
	}

	go srv.ListenAndServe()
	time.Sleep(time.Second)
} // func TestServerCreate(t *testing.T)
