// /home/krylon/go/src/github.com/blicero/badnews/web/ajax_data.go
// -*- mode: go; coding: utf-8; -*-
// Created on 29. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-29 19:40:49 krylon>

package web

import "time"

// Reply is the format used to reply to AJAX requests.
type Reply struct {
	Time    time.Time
	Status  bool
	Message string
	Payload map[string]string
}
