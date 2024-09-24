// /home/krylon/go/src/github.com/blicero/badnews/logdomain/logdomain.go
// -*- mode: go; coding: utf-8; -*-
// Created on 18. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-24 15:10:15 krylon>

package logdomain

//go:generate stringer -type=ID

type ID uint8

const (
	Database ID = iota
	DBPool
	Reader
	Web
)

func AllDomains() []ID {
	return []ID{
		Database,
		DBPool,
		Reader,
		Web,
	}
} // func AllDomains() []ID
