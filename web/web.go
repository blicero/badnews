// /home/krylon/go/src/github.com/blicero/badnews/web/web.go
// -*- mode: go; coding: utf-8; -*-
// Created on 28. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-09-30 13:36:28 krylon>

// Package web provides the web interface.
package web

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sync"
	"text/template"
	"time"

	"github.com/blicero/badnews/common"
	"github.com/blicero/badnews/common/path"
	"github.com/blicero/badnews/database"
	"github.com/blicero/badnews/logdomain"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

const (
	poolSize            = 4
	bufSize             = 32768
	keyLength           = 4096
	sessionKey          = "Wer das liest, ist doof!"
	sessionNameAgent    = "TeamOrca"
	sessionNameFrontend = "Frontend"
	sessionMaxAge       = 3600 * 24 * 7 // 1 week
)

//go:embed assets
var assets embed.FS

// Server wraps the state required for the web interface
type Server struct {
	Addr      string
	log       *log.Logger
	pool      *database.Pool
	lock      sync.RWMutex // nolint: unused,structcheck
	router    *mux.Router
	tmpl      *template.Template
	web       http.Server
	mimeTypes map[string]string
	store     sessions.Store // nolint: unused,structcheck
}

// Create creates and returns a new Server.
func Create(addr string) (*Server, error) {
	var (
		key1 = []byte(sessionKey)
		key2 = []byte(sessionKey)
	)

	slices.Reverse(key2)

	var (
		err error
		msg string
		srv = &Server{
			Addr: addr,
			mimeTypes: map[string]string{
				".css":  "text/css",
				".map":  "application/json",
				".js":   "text/javascript",
				".png":  "image/png",
				".jpg":  "image/jpeg",
				".jpeg": "image/jpeg",
				".webp": "image/webp",
				".gif":  "image/gif",
				".json": "application/json",
				".html": "text/html",
			},
			store: sessions.NewFilesystemStore(
				common.Path(path.SessionStore),
				key1,
				key2,
			),
		}
	)

	srv.store.(*sessions.FilesystemStore).MaxAge(sessionMaxAge)

	if srv.log, err = common.GetLogger(logdomain.Web); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"Error creating Logger: %s\n",
			err.Error())
		return nil, err
	} else if srv.pool, err = database.NewPool(poolSize); err != nil {
		srv.log.Printf("[ERROR] Cannot allocate database connection pool: %s\n",
			err.Error())
		return nil, err
	} else if srv.pool == nil {
		srv.log.Printf("[CANTHAPPEN] Database pool is nil!\n")
		return nil, errors.New("Database pool is nil")
	}

	const tmplFolder = "assets/templates"
	var templates []fs.DirEntry
	var tmplRe = regexp.MustCompile("[.]tmpl$")

	if templates, err = assets.ReadDir(tmplFolder); err != nil {
		srv.log.Printf("[ERROR] Cannot read embedded templates: %s\n",
			err.Error())
		return nil, err
	}

	srv.tmpl = template.New("").Funcs(funcmap)
	for _, entry := range templates {
		var (
			content []byte
			path    = filepath.Join(tmplFolder, entry.Name())
		)

		if !tmplRe.MatchString(entry.Name()) {
			continue
		} else if content, err = assets.ReadFile(path); err != nil {
			msg = fmt.Sprintf("Cannot read embedded file %s: %s",
				path,
				err.Error())
			srv.log.Printf("[CRITICAL] %s\n", msg)
			return nil, errors.New(msg)
		} else if srv.tmpl, err = srv.tmpl.Parse(string(content)); err != nil {
			msg = fmt.Sprintf("Could not parse template %s: %s",
				entry.Name(),
				err.Error())
			srv.log.Println("[CRITICAL] " + msg)
			return nil, errors.New(msg)
		} else if common.Debug {
			srv.log.Printf("[TRACE] Template \"%s\" was parsed successfully.\n",
				entry.Name())
		}
	}

	srv.router = mux.NewRouter()
	srv.web.Addr = addr
	srv.web.ErrorLog = srv.log
	srv.web.Handler = srv.router

	// Web interface handlers
	srv.router.HandleFunc("/favicon.ico", srv.handleFavIco)
	srv.router.HandleFunc("/static/{file}", srv.handleStaticFile)
	srv.router.HandleFunc("/{page:(?:index|main|start)?$}", srv.handleMain)

	// AJAX Handlers
	srv.router.HandleFunc("/ajax/beacon", srv.handleBeacon)

	return srv, nil
} // func Create(addr string) (*Server, error)

// ListenAndServe runs the server's  ListenAndServe method
func (srv *Server) ListenAndServe() {
	srv.log.Printf("[DEBUG] Server start listening on %s.\n", srv.Addr)
	defer srv.log.Println("[DEBUG] Server has quit.")
	srv.web.ListenAndServe() // nolint: errcheck
} // func (srv *Server) ListenAndServe()

func (srv *Server) handleFavIco(w http.ResponseWriter, request *http.Request) {
	srv.log.Printf("[TRACE] Handle request for %s\n",
		request.URL.EscapedPath())

	const (
		filename = "assets/static/favicon.ico"
		mimeType = "image/vnd.microsoft.icon"
	)

	w.Header().Set("Content-Type", mimeType)

	if !common.Debug {
		w.Header().Set("Cache-Control", "max-age=7200")
	} else {
		w.Header().Set("Cache-Control", "no-store, max-age=0")
	}

	var (
		err error
		fh  fs.File
	)

	if fh, err = assets.Open(filename); err != nil {
		msg := fmt.Sprintf("ERROR - cannot find file %s", filename)
		srv.sendErrorMessage(w, msg)
	} else {
		defer fh.Close()
		w.WriteHeader(200)
		io.Copy(w, fh) // nolint: errcheck
	}
} // func (srv *Server) handleFavIco(w http.ResponseWriter, request *http.Request)

func (srv *Server) handleStaticFile(w http.ResponseWriter, request *http.Request) {
	// srv.log.Printf("[TRACE] Handle request for %s\n",
	// 	request.URL.EscapedPath())

	// Since we control what static files the server has available,
	// we can easily map MIME type to slice. Soon.

	vars := mux.Vars(request)
	filename := vars["file"]
	path := filepath.Join("assets", "static", filename)

	var mimeType string

	srv.log.Printf("[TRACE] Delivering static file %s to client\n", filename)

	var match []string

	if match = common.SuffixPattern.FindStringSubmatch(filename); match == nil {
		mimeType = "text/plain"
	} else if mime, ok := srv.mimeTypes[match[1]]; ok {
		mimeType = mime
	} else {
		srv.log.Printf("[ERROR] Did not find MIME type for %s\n", filename)
	}

	w.Header().Set("Content-Type", mimeType)

	if common.Debug {
		w.Header().Set("Cache-Control", "no-store, max-age=0")
	} else {
		w.Header().Set("Cache-Control", "max-age=7200")
	}

	var (
		err error
		fh  fs.File
	)

	if fh, err = assets.Open(path); err != nil {
		msg := fmt.Sprintf("ERROR - cannot find file %s", path)
		srv.sendErrorMessage(w, msg)
	} else {
		defer fh.Close()
		w.WriteHeader(200)
		io.Copy(w, fh) // nolint: errcheck
	}
} // func (srv *Server) handleStaticFile(w http.ResponseWriter, request *http.Request)

func (srv *Server) sendErrorMessage(w http.ResponseWriter, msg string) {
	html := `
<!DOCTYPE html>
<html>
  <head>
    <title>Internal Error</title>
  </head>
  <body>
    <h1>Internal Error</h1>
    <hr />
    We are sorry to inform you an internal application error has occured:<br />
    %s
    <p>
    Back to <a href="/index">Homepage</a>
    <hr />
    &copy; 2018 <a href="mailto:krylon@gmx.net">Benjamin Walkenhorst</a>
  </body>
</html>
`

	srv.log.Printf("[ERROR] %s\n", msg)

	output := fmt.Sprintf(html, msg)
	w.WriteHeader(500)
	_, _ = w.Write([]byte(output)) // nolint: gosec
} // func (srv *Server) sendErrorMessage(w http.ResponseWriter, msg string)

func (srv *Server) handleMain(w http.ResponseWriter, r *http.Request) {
	srv.log.Printf("[TRACE] Handle request for %s from %s\n",
		r.URL.EscapedPath(),
		r.RemoteAddr)
	const tmplName = "main"
	var (
		err  error
		msg  string
		tmpl *template.Template
		db   *database.Database
		sess *sessions.Session
		data = tmplDataIndex{
			tmplDataBase: tmplDataBase{
				Title: "Main",
				Debug: true,
				URL:   r.URL.EscapedPath(),
			},
		}
	)

	db = srv.pool.Get()
	defer srv.pool.Put(db)

	if sess, err = srv.store.Get(r, sessionNameFrontend); err != nil {
		msg = fmt.Sprintf("Error getting client session from session store: %s",
			err.Error())
		srv.log.Println("[CRITICAL] " + msg)
		srv.sendErrorMessage(w, msg)
		return
	} else if tmpl = srv.tmpl.Lookup(tmplName); tmpl == nil {
		msg = fmt.Sprintf("Could not find template %q", tmplName)
		srv.log.Println("[CRITICAL] " + msg)
		srv.sendErrorMessage(w, msg)
		return
	} else if data.Feeds, err = db.FeedGetAll(); err != nil {
		msg = fmt.Sprintf("Failed to load Feeds: %s", err.Error())
		srv.log.Println("[CRITICAL] " + msg)
		srv.sendErrorMessage(w, msg)
		return
	}

	if err = sess.Save(r, w); err != nil {
		srv.log.Printf("[ERROR] Failed to set session cookie: %s\n",
			err.Error())
	}
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(200)
	if err = tmpl.Execute(w, &data); err != nil {
		msg = fmt.Sprintf("Error rendering template %q: %s",
			tmplName,
			err.Error())
		srv.sendErrorMessage(w, msg)
	}
} // func (srv *Server) handleMain(w http.ResponseWriter, r *http.Request)

////////////////////////////////////////////////////////////////////////////////
//// Ajax handlers /////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////

// const success = "Success"

func (srv *Server) handleBeacon(w http.ResponseWriter, r *http.Request) {
	// srv.log.Printf("[TRACE] Handle %s from %s\n",
	// 	r.URL,
	// 	r.RemoteAddr)
	var timestamp = time.Now().Format(common.TimestampFormat)
	const appName = common.AppName + " " + common.Version
	var jstr = fmt.Sprintf(`{ "Status": true, "Message": "%s", "Timestamp": "%s", "Hostname": "%s" }`,
		appName,
		timestamp,
		hostname())
	var response = []byte(jstr)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.WriteHeader(200)
	w.Write(response) // nolint: errcheck,gosec
} // func (srv *Web) handleBeacon(w http.ResponseWriter, r *http.Request)