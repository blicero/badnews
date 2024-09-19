// /home/krylon/go/src/github.com/blicero/scrollmaster/common/01_common_test.go
// -*- mode: go; coding: utf-8; -*-
// Created on 04. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-04 14:06:54 krylon>

package common

import "testing"

func TestFibonacci(t *testing.T) {
	type testCase struct {
		input          int64
		expectedResult int64
	}

	var tests = []testCase{
		{1, 1},
		{3, 2},
		{5, 5},
		{10, 55},
		{20, 6765},
	}

	for _, c := range tests {
		var res int64 = Fibonacci(c.input)

		if res != c.expectedResult {
			t.Errorf("Unexpected result for Fibonacci(%d): %d (exptected %d)",
				c.input,
				res,
				c.expectedResult)
		}
	}
} // func TestFibonacci(t *testing.T)
