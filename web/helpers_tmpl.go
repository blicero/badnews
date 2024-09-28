// /home/krylon/go/src/pepper/web/helpers.go
// -*- mode: go; coding: utf-8; -*-
// Created on 12. 12. 2018 by Benjamin Walkenhorst
// (c) 2018 Benjamin Walkenhorst
// Time-stamp: <2024-09-28 15:53:04 krylon>

package web

import (
	"errors"
	"fmt"
	"html"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/blicero/badnews/common"

	"github.com/mborgerson/GoTruncateHtml/truncatehtml"
)

////////////////////////////////////
// Functions for use in templates //
////////////////////////////////////

var funcmap = template.FuncMap{
	"sequence":         sequenceFunc,
	"cycle":            cycleFunc,
	"new_counter":      newCounter,
	"now":              nowFunc,
	"app_string":       appStringFunc,
	"app_build":        appBuildFunc,
	"hostname":         hostname,
	"fmt_bytes":        formatBytes,
	"fmt_time":         formatTime,
	"fmt_time_form":    formatTimeForm,
	"fmt_time_minute":  formatTimeMinute,
	"fmt_float":        formatFloat,
	"current_year":     currentYear,
	"minutes":          minutes,
	"lower":            lower,
	"sanitize":         sanitize,
	"argstring":        argString,
	"isnil":            isNil,
	"notnil":           notNil,
	"fmt_script_args":  argsString,
	"join":             joinStrings,
	"escape_linebreak": escapeLinebreak,
	"nbsp":             nbsp,
	"uuid":             common.GetUUID,
	"concat":           concat,
	"i64str":           i64str,
	"truncate":         truncateHTML,
	"intRange":         intRange,
	"inc":              inc,
	"dec":              dec,
}

type generator struct {
	values []string
	index  int
	f      func(s []string, i int) string
}

func (seq *generator) Next() string {
	s := seq.f(seq.values, seq.index)
	seq.index++
	return s
} // func (seq *generator) Next() string

type counter struct {
	c int
}

func newCounter() *counter {
	return &counter{c: 0}
} // func newCounter() counter

func (c *counter) Next() string {
	c.c++
	return strconv.Itoa(c.c)
} // func (c counter) Next() counter

func sequenceGen(values []string, i int) string {
	if i >= len(values) {
		return values[len(values)-1]
	}

	return values[i]
} // func sequenceGen(values []string, i int) string

func cycleGen(values []string, i int) string {
	return values[i%len(values)]
} // func cycleGen(values []string, i int) string

func sequenceFunc(values ...string) (*generator, error) {
	if len(values) == 0 {
		return nil, errors.New("Sequence must have at least one element")
	}

	return &generator{
		values: values,
		index:  0,
		f:      sequenceGen,
	}, nil
} // func sequenceFunc(values ...string) (*generator, error)

func cycleFunc(values ...string) (*generator, error) {
	if len(values) == 0 {
		return nil, errors.New("Cycle must have at least one element")
	}

	return &generator{
		values: values,
		index:  0,
		f:      cycleGen,
	}, nil
} // func cycleFunc(values ...string) (*generator, error)

func nowFunc() string {
	return time.Now().Format(common.TimestampFormat)
} // func nowFunc() string

func appStringFunc() string {
	return fmt.Sprintf("%s %s",
		common.AppName,
		common.Version)
} // func appStringFunc() string

func appBuildFunc() string {
	return common.BuildStamp.Format("2006-01-02 15:04:05 MST")
} // func appBuildFunc() string

func formatBytes(n int64) string {
	if n < 0 {
		return ""
	}

	var units = []string{
		"Bytes",
		"KiB",
		"MiB",
		"GiB",
		"TiB",
		"PiB",
		"EiB",
	}
	var idx = 0
	var amount = float64(n)

	for amount > 1024 {
		amount /= 1024
		idx++
	}

	return fmt.Sprintf("%.2f %s",
		amount,
		units[idx])
} // func formatBytes(int64 n) string

func formatTime(t time.Time) string {
	return t.Format(common.TimestampFormat)
} // func formatTime(t time.Time) string

func formatTimeMinute(t time.Time) string {
	return t.Format(common.TimestampFormatMinute)
} // func formatTimeMinute(t time.Time) string

func formatTimeForm(t time.Time) string {
	return t.Format(common.TimestampFormatForm)
} // func formatTimeForm(t time.Time) string

func formatFloat(f float64) string {
	return fmt.Sprintf("%.1f", f)
} // func formatFloat(f float64) string

func currentYear() string {
	var year = time.Now().Year()
	return strconv.Itoa(year)
} // func currentYear() string

func minutes(d time.Duration) int {
	return int(d.Minutes())
} // func minutes(d time.Duration) int

func hostname() string {
	var (
		name string
		err  error
	)

	if name, err = os.Hostname(); err != nil {
		return "<hostname>"
	}

	return name
} // func hostname() string

func lower(input string) string {
	return strings.ToLower(input)
} // func lower(input string) string

func sanitize(input string) string {
	return html.EscapeString(input)
} // func sanitize(input string) string

func argString(args []string) string {
	var qlist = make([]string, len(args))

	for i, s := range args {
		qlist[i] = "\"" + s + "\""
	}

	return strings.Join(qlist, " ")
} // func argString(args []string) string

func isNil(arg interface{}) bool {
	return arg == nil
} // func isNil(arg interface{}) bool

func notNil(arg interface{}) bool {
	return arg != nil
} // func notNil(arg interface{}) bool

func argsString(args map[string]string) string {
	var pairs = make([]string, 0, len(args))

	for k, v := range args {
		var s = fmt.Sprintf("%s=%s",
			k,
			v)
		pairs = append(pairs, s)
	}

	var result = strings.Join(pairs, ", ")
	return result
} // func argsString(map[string]string) string

func joinStrings(arr []string, sep string, quote bool) string {
	if quote {
		var quoted = make([]string, len(arr))

		for idx, val := range arr {
			quoted[idx] = `"` + val + `"`
		}
		return strings.Join(quoted, sep)
	}

	return strings.Join(arr, sep)
} // func joinStrings(arr []string) string

var newline = regexp.MustCompile("[\r\n]")

func escapeLinebreak(str string) string {
	return newline.ReplaceAllString(str, "\\n")
} // func escapeLinebreak(str string) string

func nbsp(cnt int) string {
	const entity = "&nbsp;"

	var bld strings.Builder

	bld.Grow(len(entity)*cnt + 2)

	for i := 0; i < cnt; i++ {
		bld.WriteString(entity) // nolint: errcheck,gosec
	}

	return bld.String()
} // func nbsp(cnt int) string

func concat(s1, s2 string) string {
	return s1 + s2
} // func concat(s1, s2 string) string

func i64str(i int64) string {
	return strconv.FormatInt(i, 10)
} // func i64str(i int64) string

func truncateHTML(s string, maxlen int) string {
	const ellipsis = "..."
	var (
		err    error
		output []byte
		input  = []byte(s)
	)

	if len(s) <= maxlen {
		return s
	} else if output, err = truncatehtml.TruncateHtml(input, maxlen, ellipsis); err != nil {
		fmt.Fprintf(os.Stderr,
			"Cannot truncate HTML: %s\n",
			err.Error())
		return s
	}

	return string(output)
} // func truncateHTML(s string, maxlen int) string

func intRange(n int64) []int64 {
	var (
		i    int64
		list = make([]int64, n)
	)

	for i = 0; i < n; i++ {
		list[i] = i
	}

	return list
} // func intRange(n int64) []int64

func inc(n int64) int64 {
	return n + 1
} // func inc(n int64) int64

func dec(n int64) int64 {
	return n - 1
} // func dec(n int64) int64
