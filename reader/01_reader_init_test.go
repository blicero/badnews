// /home/krylon/go/src/github.com/blicero/badnews/reader/01_reader_init_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 26. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-26 20:07:09 krylon>

package reader

import "testing"

func TestReaderNew(t *testing.T) {
	var err error

	if rdr, err = New(2); err != nil {
		rdr = nil
		t.Fatalf("Error creating new Reader: %s",
			err.Error())
	} else if rdr.IsActive() {
		t.Fatal("Newly created Reader should not be active")
	}
} // func TestReaderNew(t *testing.T)
