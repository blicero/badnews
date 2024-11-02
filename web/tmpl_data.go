// /home/krylon/go/src/github.com/blicero/server/tmpl_data.go
// -*- mode: go; coding: utf-8; -*-
// Created on 06. 05. 2020 by Benjamin Walkenhorst
// (c) 2020 Benjamin Walkenhorst
// Time-stamp: <2024-11-02 20:10:46 krylon>
//
// This file contains data structures to be passed to HTML templates.

package web

import (
	"crypto/sha512"
	"fmt"
	"time"

	"github.com/blicero/badnews/advisor"
	"github.com/blicero/badnews/blacklist"
	"github.com/blicero/badnews/common"
	"github.com/blicero/badnews/model"

	"github.com/hashicorp/logutils"
)

type message struct { // nolint: unused
	Timestamp time.Time
	Level     logutils.LogLevel
	Message   string
}

func (m *message) TimeString() string { // nolint: unused
	return m.Timestamp.Format(common.TimestampFormat)
} // func (m *Message) TimeString() string

func (m *message) Checksum() string { // nolint: unused
	var str = m.Timestamp.Format(common.TimestampFormat) + "##" +
		string(m.Level) + "##" +
		m.Message

	var hash = sha512.New()
	hash.Write([]byte(str)) // nolint: gosec,errcheck

	var cksum = hash.Sum(nil)
	var ckstr = fmt.Sprintf("%x", cksum)

	return ckstr
} // func (m *message) Checksum() string

type tmplDataBase struct { // nolint: unused
	Title string
	Debug bool
	URL   string
}

type tmplDataIndex struct { // nolint: unused,deadcode
	tmplDataBase
	Feeds []model.Feed
}

type tmplDataItems struct {
	tmplDataBase
	ReqCnt      int64
	MaxItems    int64
	Feeds       map[int64]model.Feed
	Suggestions map[int64][]advisor.SuggestedTag
}

type tmplDataItemView struct {
	tmplDataBase
	Feeds       map[int64]model.Feed
	Items       []*model.Item
	Tags        []*model.Tag
	Suggestions map[int64][]advisor.SuggestedTag
}

type tmplDataFeedDetails struct {
	tmplDataBase
	Feed        *model.Feed
	Feeds       map[int64]model.Feed
	Items       []*model.Item
	Tags        []*model.Tag
	Suggestions map[int64][]advisor.SuggestedTag
}

type tmplDataTagForm struct {
	tmplDataBase
	Tags []*model.Tag
	Tag  model.Tag
}

type tmplDataTagAll struct {
	tmplDataBase
	Tags    []*model.Tag
	ItemCnt map[int64]int64
	Tag     model.Tag
}

type tmplDataBlacklist struct {
	tmplDataBase
	Blacklist *blacklist.Blacklist
}

// Local Variables:  //
// compile-command: "go generate && go vet && go build -v -p 16 && gometalinter && go test -v" //
// End: //
