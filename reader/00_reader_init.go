// /home/krylon/go/src/github.com/blicero/badnews/reader/00_reader_init.go
// -*- mode: go; coding: utf-8; -*-
// Created on 26. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-26 19:58:10 krylon>

package reader

import (
	"net/url"
	"time"

	"github.com/blicero/badnews/common"
	"github.com/blicero/badnews/common/path"
	"github.com/blicero/badnews/database"
	"github.com/blicero/badnews/model"
)

var rdr *Reader

func purl(s string) *url.URL {
	var (
		err error
		u   *url.URL
	)

	if u, err = url.Parse(s); err != nil {
		panic(err)
	}

	return u
} // func purl(s string) *url.URL

var testFeeds = []*model.Feed{
	{
		Title:          "DLF Nachrichten",
		URL:            purl("https://www.deutschlandfunk.de/die-nachrichten.353.de.rss"),
		Homepage:       purl("https://www.dlf.de/"),
		UpdateInterval: time.Second * 10,
		Active:         true,
	},
	{
		Title:          "Tagesschau Nachrichten",
		URL:            purl("http://www.tagesschau.de/xml/rss2"),
		Homepage:       purl("https://www.tagesschau.de/"),
		UpdateInterval: time.Second * 10,
		Active:         true,
	},
	{
		Title:          "WDR Nachrichten Bielefeld",
		URL:            purl("https://www1.wdr.de/nachrichten/bielefeld-nachrichten-100.feed"),
		Homepage:       purl("https://www1.wdr.de/nachrichten/"),
		UpdateInterval: time.Second * 10,
		Active:         true,
	},
}

func prepare() error {
	var (
		err error
		db  *database.Database
	)

	if db, err = database.Open(common.Path(path.Database)); err != nil {
		return err
	}

	db.Begin() // nolint: errcheck

	for _, f := range testFeeds {
		if err = db.FeedAdd(f); err != nil {
			db.Rollback() // nolint: errcheck
			return err
		}
	}

	db.Commit() // nolint: errcheck

	return nil
} // func prepare() error
