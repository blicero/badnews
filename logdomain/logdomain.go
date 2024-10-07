// /home/krylon/go/src/github.com/blicero/badnews/logdomain/logdomain.go
// -*- mode: go; coding: utf-8; -*-
// Created on 18. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-10-04 20:49:04 krylon>

package logdomain

//go:generate stringer -type=ID

type ID uint8

const (
	Database ID = iota
	DBPool
	Judge
	Reader
	Web
)

func AllDomains() []ID {
	return []ID{
		Database,
		DBPool,
		Judge,
		Reader,
		Web,
	}
} // func AllDomains() []ID
