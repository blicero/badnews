// /home/krylon/go/src/github.com/blicero/badnews/web/ajax_data.go
// -*- mode: go; coding: utf-8; -*-
// Created on 29. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-30 18:54:30 krylon>

package web

import "time"

// Reply is the format used to reply to AJAX requests.
type Reply struct {
	Timestamp time.Time         `json:"time"`
	Status    bool              `json:"status"`
	Message   string            `json:"message"`
	Payload   map[string]string `json:"payload"`
}
