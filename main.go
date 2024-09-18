// /home/krylon/go/src/github.com/blicero/badnews/main.go
// -*- mode: go; coding: utf-8; -*-
// Created on 18. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-18 21:09:45 krylon>

package main

import (
	"fmt"

	"github.com/blicero/badnews/common"
)

func main() {
	fmt.Printf("%s %s built on %s\n",
		common.AppName,
		common.Version,
		common.BuildStamp.Format(common.TimestampFormat))
	fmt.Println("IMPLEMENT ME!")
}
