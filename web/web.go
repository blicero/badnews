// /home/krylon/go/src/github.com/blicero/badnews/web/web.go
// -*- mode: go; coding: utf-8; -*-
// Created on 28. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-10-24 19:57:34 krylon>

// Package web provides the web interface.
package web

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"sync"
	"text/template"
	"time"

	"github.com/blicero/badnews/advisor"
	"github.com/blicero/badnews/common"
	"github.com/blicero/badnews/common/path"
	"github.com/blicero/badnews/database"
	"github.com/blicero/badnews/judge"
	"github.com/blicero/badnews/logdomain"
	"github.com/blicero/badnews/model"
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
	suggPerItem         = 10
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
	store     sessions.Store
	judge     *judge.Judge
	adv       *advisor.Advisor
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
	} else if srv.judge, err = judge.New(); err != nil {
		srv.log.Printf("[ERROR] Failed to create Judge: %s\n",
			err.Error())
		srv.pool.Close() // nolint: errcheck
		return nil, err
	} else if err = srv.judge.Train(); err != nil {
		srv.log.Printf("[CRITICAL] Failed to train classifier: %s\n",
			err.Error())
		return nil, err
	} else if srv.adv, err = advisor.NewAdvisor(); err != nil {
		srv.log.Printf("[CRITICAL] Failed to create Advisor: %s\n",
			err.Error())
		return nil, err
	} else if err = srv.adv.Train(); err != nil {
		srv.log.Printf("[CRITICAL] Failed to train Advisor: %s\n",
			err.Error())
		return nil, err
	}

	// TODO As shield uses a database to persists its training data, I don't
	//      really need to train it on startup, but for the moment, I do that
	//      anyway. Once I'm happy with how everything works, I can skip that
	//      step.

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
	srv.router.HandleFunc("/items/{cnt:(?:\\d+)}", srv.handleItemPage)
	srv.router.HandleFunc("/feed/{id:(?:\\d+$)}", srv.handleFeedDetails)
	srv.router.HandleFunc("/tags/all", srv.handleTagAll)

	// AJAX Handlers
	srv.router.HandleFunc("/ajax/beacon", srv.handleBeacon)
	srv.router.HandleFunc("/ajax/subscribe", srv.handleSubscribe)
	srv.router.HandleFunc("/ajax/items/{offset:(?:\\d+)}/{cnt:(?:\\d+)}", srv.handleAjaxItems)
	srv.router.HandleFunc("/ajax/feed_items/{id:(?:\\d+)$}", srv.handleAjaxItemsByFeed)
	srv.router.HandleFunc("/ajax/item_rate", srv.handleAjaxRateItem)
	srv.router.HandleFunc("/ajax/item_unrate/{id:(?:\\d+)$}", srv.handleAjaxUnrateItem)
	srv.router.HandleFunc("/ajax/tag/all", srv.handleAjaxTagView)
	srv.router.HandleFunc("/ajax/tag/submit", srv.handleAjaxTagSubmit)
	srv.router.HandleFunc("/ajax/tag/details/{id:(?:\\d+)$}", srv.handleAjaxTagDetails)
	srv.router.HandleFunc("/ajax/tag/link/{tag:(?:\\d+)}/{item:(?:\\d+)}", srv.handleAjaxTagLinkAdd)
	srv.router.HandleFunc("/ajax/tag/unlink/{tag:(?:\\d+)}/{item:(?:\\d+)}", srv.handleAjaxTagLinkRemove)

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

func (srv *Server) handleItemPage(w http.ResponseWriter, r *http.Request) {
	const tmplName = "items"
	srv.log.Printf("[TRACE] Handle request for %s from %s\n",
		r.URL.EscapedPath(),
		r.RemoteAddr)
	var (
		err  error
		msg  string
		tmpl *template.Template
		db   *database.Database
		sess *sessions.Session
		data = tmplDataItems{
			tmplDataBase: tmplDataBase{
				Title: "Items",
				Debug: true,
				URL:   r.URL.EscapedPath(),
			},
			ReqCnt: 25,
		}
		vars  map[string]string
		feeds []model.Feed
	)

	vars = mux.Vars(r)

	if data.MaxItems, err = strconv.ParseInt(vars["cnt"], 10, 64); err != nil {
		msg = fmt.Sprintf("Cannot parse item count %q: %s",
			vars["cnt"],
			err.Error())
		srv.log.Printf("[CANTHAPPEN] %s\n",
			msg)
		srv.sendErrorMessage(w, msg)
		return
	}

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
	} else if feeds, err = db.FeedGetAll(); err != nil {
		msg = fmt.Sprintf("Failed to load Feeds: %s", err.Error())
		srv.log.Println("[CRITICAL] " + msg)
		srv.sendErrorMessage(w, msg)
		return
	}

	data.Feeds = make(map[int64]model.Feed, len(feeds))

	for _, f := range feeds {
		data.Feeds[f.ID] = f
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
} // func (srv *Server) handleItemPage(w http.ResponseWriter, r *http.Request)

func (srv *Server) handleFeedDetails(w http.ResponseWriter, r *http.Request) {
	const tmplName = "feed_details"
	srv.log.Printf("[TRACE] Handle request for %s from %s\n",
		r.URL.EscapedPath(),
		r.RemoteAddr)
	var (
		err    error
		msg    string
		tmpl   *template.Template
		db     *database.Database
		feedID int64
		sess   *sessions.Session
		data   = tmplDataFeedDetails{
			tmplDataBase: tmplDataBase{
				Title: "Items",
				Debug: true,
				URL:   r.URL.EscapedPath(),
			},
		}
		vars map[string]string
	)

	vars = mux.Vars(r)

	if feedID, err = strconv.ParseInt(vars["id"], 10, 64); err != nil {
		msg = fmt.Sprintf("Cannot parse feed ID %q: %s",
			vars["id"],
			err.Error())
		srv.log.Printf("[CANTHAPPEN] %s\n",
			msg)
		srv.sendErrorMessage(w, msg)
		return
	}

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
	} else if data.Feed, err = db.FeedGetByID(feedID); err != nil {
		msg = fmt.Sprintf("Failed to load Feed %d: %s", feedID, err.Error())
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
} // func (srv *Server) handleFeedDetails(w http.ResponseWriter, r *http.Request)

func (srv *Server) handleTagAll(w http.ResponseWriter, r *http.Request) {
	const tmplName = "tags"
	srv.log.Printf("[TRACE] Handle request for %s from %s\n",
		r.URL.EscapedPath(),
		r.RemoteAddr)
	var (
		err  error
		msg  string
		tmpl *template.Template
		sess *sessions.Session
		db   *database.Database
		data = tmplDataTagAll{
			tmplDataBase: tmplDataBase{
				Title: "Items",
				Debug: true,
				URL:   r.URL.EscapedPath(),
			},
		}
	)

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
	}

	db = srv.pool.Get()
	defer srv.pool.Put(db)

	if data.Tags, err = db.TagGetAll(); err != nil {
		msg = fmt.Sprintf("Failed to load Tags: %s",
			err.Error())
		srv.log.Println("[CRITICAL] " + msg)
		srv.sendErrorMessage(w, msg)
		return
	} else if data.ItemCnt, err = db.TagGetItemCnt(); err != nil {
		msg = fmt.Sprintf("Failed to load Item count: %s",
			err.Error())
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
} // func (srv *Server) handleTagAll(w http.ResponseWriter, r *http.Request)

// func (srv *Server) handleTagDetails(w http.ResponseWriter, r *http.Request) {
// 	const tmplName = "
// } // func (srv *handleTagDetails(w http.ResponseWriter, r *http.Request)

////////////////////////////////////////////////////////////////////////////////
//// Ajax handlers /////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////

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

func (srv *Server) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	srv.log.Printf("[TRACE] Handle request for %s from %s\n",
		r.URL.EscapedPath(),
		r.RemoteAddr)
	var (
		err      error
		sess     *sessions.Session
		feed     model.Feed
		rbuf     []byte
		db       *database.Database
		interval int64
		res      Reply
		msg      string
		hstatus  = 200
	)

	if sess, err = srv.store.Get(r, sessionNameFrontend); err != nil {
		res.Message = fmt.Sprintf(
			"Error getting/creating session %s: %s",
			sessionNameFrontend,
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		sess = nil
		hstatus = 403
		goto SEND_RESPONSE
	} else if err = r.ParseForm(); err != nil {
		res.Message = fmt.Sprintf("Cannot parse form data: %s",
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 400
		goto SEND_RESPONSE
	} else if feed.URL, err = url.Parse(r.FormValue("url")); err != nil {
		res.Message = fmt.Sprintf("Cannot parse URL %q: %s",
			r.FormValue("url"),
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	} else if feed.Homepage, err = url.Parse(r.FormValue("homepage")); err != nil {
		res.Message = fmt.Sprintf("Cannot parse Homepage %q: %s",
			r.FormValue("homepage"),
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	} else if interval, err = strconv.ParseInt(r.FormValue("interval"), 10, 64); err != nil {
		res.Message = fmt.Sprintf("Cannot parse refresh interval %q: %s",
			r.FormValue("interval"),
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	}

	feed.Title = r.FormValue("title")
	feed.UpdateInterval = time.Second * time.Duration(interval)
	db = srv.pool.Get()
	defer srv.pool.Put(db)

	if err = db.FeedAdd(&feed); err != nil {
		res.Message = fmt.Sprintf("Error adding feed %s to database: %s",
			feed.Title,
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	}

	res.Message = fmt.Sprintf("Successfully added Feed %s to database",
		feed.Title)
	res.Status = true
	res.Payload = map[string]string{
		"id": strconv.Itoa(int(feed.ID)),
	}

SEND_RESPONSE:
	if sess != nil {
		if err = sess.Save(r, w); err != nil {
			srv.log.Printf("[ERROR] Failed to set session cookie: %s\n",
				err.Error())
		}
	}
	res.Timestamp = time.Now()
	if rbuf, err = json.Marshal(&res); err != nil {
		srv.log.Printf("[ERROR] Error serializing response: %s\n",
			err.Error())
		rbuf = errJSON(err.Error())
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.WriteHeader(hstatus)
	if _, err = w.Write(rbuf); err != nil {
		msg = fmt.Sprintf("Failed to send result: %s",
			err.Error())
		srv.log.Println("[ERROR] " + msg)
	}
} // func (srv *Server) handleSubscribe(w http.ResponseWriter, r *http.Request)

func (srv *Server) handleAjaxItems(w http.ResponseWriter, r *http.Request) {
	srv.log.Printf("[TRACE] Handle request for %s from %s\n",
		r.URL.EscapedPath(),
		r.RemoteAddr)

	const tmplName = "item_view"
	var (
		err         error
		sess        *sessions.Session
		rbuf        []byte
		db          *database.Database
		buf         bytes.Buffer
		tmpl        *template.Template
		cnt, offset int64
		res         Reply
		msg, rating string
		feeds       []model.Feed
		tags        []*model.Tag
		vars        map[string]string
		hstatus     = 200
		data        = tmplDataItemView{
			tmplDataBase: tmplDataBase{
				Debug: common.Debug,
				URL:   r.URL.String(),
			},
		}
	)

	vars = mux.Vars(r)

	if sess, err = srv.store.Get(r, sessionNameFrontend); err != nil {
		msg = fmt.Sprintf("Error getting client session from session store: %s",
			err.Error())
		srv.log.Println("[CRITICAL] " + msg)
		srv.sendErrorMessage(w, msg)
		return
	} else if cnt, err = strconv.ParseInt(vars["cnt"], 10, 64); err != nil {
		res.Message = fmt.Sprintf("Cannot parse item count %q: %s",
			vars["cnt"],
			err.Error())
		srv.log.Printf("[CANTHAPPEN] %s\n", res.Message)
		hstatus = 400
		goto SEND_RESPONSE
	} else if offset, err = strconv.ParseInt(vars["offset"], 10, 64); err != nil {
		res.Message = fmt.Sprintf("Cannot parse offset %q: %s",
			vars["offset"],
			err.Error())
		srv.log.Printf("[CANTHAPPEN] %s\n", res.Message)
		hstatus = 400
		goto SEND_RESPONSE
	}

	srv.log.Printf("[DEBUG] Load %d items, offset %d",
		cnt,
		offset)

	db = srv.pool.Get()
	defer srv.pool.Put(db)

	if data.Items, err = db.ItemGetRecentPaged(cnt, offset); err != nil {
		res.Message = fmt.Sprintf("Failed to load recent items: %s",
			err.Error())
		srv.log.Printf("[CANTHAPPEN] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	} else if feeds, err = db.FeedGetAll(); err != nil {
		res.Message = fmt.Sprintf("Failed to load all Feeds from database: %s",
			err.Error())
		srv.log.Printf("[CANTHAPPEN] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	} else if tags, err = db.TagGetSorted(); err != nil {
		res.Message = fmt.Sprintf("Failed to load all Tags: %s",
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	} else if tmpl = srv.tmpl.Lookup(tmplName); tmpl == nil {
		res.Message = fmt.Sprintf("Could not find template %q", tmplName)
		srv.log.Printf("[CANTHAPPEN] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	}

	data.Suggestions = make(map[int64][]advisor.SuggestedTag, len(data.Items))

	for _, i := range data.Items {
		if i.Tags, err = db.TagLinkGetByItem(i); err != nil {
			res.Message = fmt.Sprintf("Failed to load linked tags for Item %d: %s",
				i.ID,
				err.Error())
			srv.log.Printf("[ERROR] %s\n", res.Message)
			hstatus = 500
			goto SEND_RESPONSE
		} else if i.EffectiveRating() == 0 {
			srv.log.Printf("[TRACE] Using classifier to guess rating for item %q (%d)\n",
				i.Headline,
				i.ID)
			if rating, err = srv.judge.Rate(i); err != nil {
				srv.log.Printf("[ERROR] Failed to rate Item %q (%d): %s\n",
					i.Headline,
					i.ID,
					err.Error())
				continue
			}

			srv.log.Printf("[TRACE] Item %q (%d) has been classified as %q\n",
				i.Headline,
				i.ID,
				rating)

			switch rating {
			case "boring":
				i.Guessed = -1
			case "interesting":
				i.Guessed = 1
			}
		}

		data.Suggestions[i.ID] = srv.adv.Suggest(i, suggPerItem)
	}

	data.Feeds = make(map[int64]model.Feed, len(feeds))
	data.Tags = make(map[int64]*model.Tag, len(tags))

	for _, f := range feeds {
		data.Feeds[f.ID] = f
	}

	for _, t := range tags {
		data.Tags[t.ID] = t
	}

	if err = tmpl.Execute(&buf, &data); err != nil {
		res.Message = fmt.Sprintf("Failed to render template %q: %s",
			tmplName,
			err.Error())
		srv.log.Printf("[CANTHAPPEN] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	}

	res.Payload = map[string]string{
		"content": buf.String(),
		"count":   strconv.Itoa(len(data.Items)),
	}
	res.Status = true

SEND_RESPONSE:
	if sess != nil {
		if err = sess.Save(r, w); err != nil {
			srv.log.Printf("[ERROR] Failed to set session cookie: %s\n",
				err.Error())
		}
	}
	res.Timestamp = time.Now()
	if rbuf, err = json.Marshal(&res); err != nil {
		srv.log.Printf("[ERROR] Error serializing response: %s\n",
			err.Error())
		rbuf = errJSON(err.Error())
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.WriteHeader(hstatus)
	if _, err = w.Write(rbuf); err != nil {
		msg = fmt.Sprintf("Failed to send result: %s",
			err.Error())
		srv.log.Println("[ERROR] " + msg)
	}
} // func (srv *Server) handleAjaxItems(w http.ResponseWriter, r *http.Request)

func (srv *Server) handleAjaxItemsByFeed(w http.ResponseWriter, r *http.Request) {
	srv.log.Printf("[TRACE] Handle request for %s from %s\n",
		r.URL.EscapedPath(),
		r.RemoteAddr)
	const (
		tmplName = "item_view"
		offset   = 0
		cnt      = 50
	)
	var (
		err         error
		sess        *sessions.Session
		rbuf        []byte
		db          *database.Database
		buf         bytes.Buffer
		tmpl        *template.Template
		feedID      int64
		res         Reply
		msg, rating string
		vars        map[string]string
		feeds       []model.Feed
		hstatus     = 200
		data        = tmplDataFeedDetails{
			tmplDataBase: tmplDataBase{
				Debug: common.Debug,
				URL:   r.URL.String(),
			},
		}
	)

	vars = mux.Vars(r)

	if sess, err = srv.store.Get(r, sessionNameFrontend); err != nil {
		msg = fmt.Sprintf("Error getting client session from session store: %s",
			err.Error())
		srv.log.Println("[CRITICAL] " + msg)
		srv.sendErrorMessage(w, msg)
		return
	} else if feedID, err = strconv.ParseInt(vars["id"], 10, 64); err != nil {
		res.Message = fmt.Sprintf("Cannot parse item count %q: %s",
			vars["cnt"],
			err.Error())
		srv.log.Printf("[CANTHAPPEN] %s\n", res.Message)
		hstatus = 400
		goto SEND_RESPONSE
	}

	db = srv.pool.Get()
	defer srv.pool.Put(db)

	if data.Feed, err = db.FeedGetByID(feedID); err != nil {
		res.Message = fmt.Sprintf("Failed to get Feed %d: %s",
			feedID,
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	} else if feeds, err = db.FeedGetAll(); err != nil {
		res.Message = fmt.Sprintf("Failed to load all Feeds from database: %s",
			err.Error())
		srv.log.Printf("[CANTHAPPEN] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	} else if data.Items, err = db.ItemGetByFeed(data.Feed, cnt, offset); err != nil {
		res.Message = fmt.Sprintf("Failed to get recent Items for Feed %s (%d): %s",
			data.Feed.Title,
			data.Feed.ID,
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	} else if tmpl = srv.tmpl.Lookup(tmplName); tmpl == nil {
		res.Message = fmt.Sprintf("Could not find template %q", tmplName)
		srv.log.Printf("[CANTHAPPEN] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	}

	data.Feeds = make(map[int64]model.Feed, len(feeds))

	for _, f := range feeds {
		data.Feeds[f.ID] = f
	}

	data.Suggestions = make(map[int64][]advisor.SuggestedTag, len(data.Items))

	for _, i := range data.Items {
		if i.EffectiveRating() == 0 {
			srv.log.Printf("[TRACE] Using classifier to guess rating for item %q (%d)\n",
				i.Headline,
				i.ID)
			if rating, err = srv.judge.Rate(i); err != nil {
				srv.log.Printf("[ERROR] Failed to rate Item %q (%d): %s\n",
					i.Headline,
					i.ID,
					err.Error())
				continue
			}

			srv.log.Printf("[TRACE] Item %q (%d) has been classified as %q\n",
				i.Headline,
				i.ID,
				rating)

			switch rating {
			case "boring":
				i.Guessed = -1
			case "interesting":
				i.Guessed = 1
			}
		}
	}

	if err = tmpl.Execute(&buf, &data); err != nil {
		res.Message = fmt.Sprintf("Failed to render template %q: %s",
			tmplName,
			err.Error())
		srv.log.Printf("[CANTHAPPEN] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	}

	res.Payload = map[string]string{
		"items": buf.String(),
		"count": strconv.Itoa(len(data.Items)),
	}
	res.Status = true

SEND_RESPONSE:
	if sess != nil {
		if err = sess.Save(r, w); err != nil {
			srv.log.Printf("[ERROR] Failed to set session cookie: %s\n",
				err.Error())
		}
	}
	res.Timestamp = time.Now()
	if rbuf, err = json.Marshal(&res); err != nil {
		srv.log.Printf("[ERROR] Error serializing response: %s\n",
			err.Error())
		rbuf = errJSON(err.Error())
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.WriteHeader(hstatus)
	if _, err = w.Write(rbuf); err != nil {
		msg = fmt.Sprintf("Failed to send result: %s",
			err.Error())
		srv.log.Println("[ERROR] " + msg)
	}
} // func (srv *Server) handleAjaxItemsByFeed(w http.ResponseWriter, r *http.Request)

func (srv *Server) handleAjaxRateItem(w http.ResponseWriter, r *http.Request) {
	srv.log.Printf("[TRACE] Handle request for %s from %s\n",
		r.URL.EscapedPath(),
		r.RemoteAddr)

	var (
		err         error
		sess        *sessions.Session
		rbuf        []byte
		db          *database.Database
		idstr, rstr string
		id, rating  int64
		item        *model.Item
		res         Reply
		msg         string
		hstatus     = 200
	)

	if sess, err = srv.store.Get(r, sessionNameFrontend); err != nil {
		msg = fmt.Sprintf("Error getting client session from session store: %s",
			err.Error())
		srv.log.Println("[CRITICAL] " + msg)
		srv.sendErrorMessage(w, msg)
		return
	} else if err = r.ParseForm(); err != nil {
		res.Message = fmt.Sprintf("Cannot parse form data: %s",
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 400
		goto SEND_RESPONSE
	}

	idstr = r.FormValue("item")
	rstr = r.FormValue("rating")

	if id, err = strconv.ParseInt(idstr, 10, 64); err != nil {
		res.Message = fmt.Sprintf("Cannot parse item ID %q: %s",
			idstr,
			err.Error())
		srv.log.Printf("[ERROR] %s\n",
			res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	} else if rating, err = strconv.ParseInt(rstr, 10, 8); err != nil {
		res.Message = fmt.Sprintf("Cannot parse rating %q: %s",
			rstr,
			err.Error())
		srv.log.Printf("[ERROR] %s\n",
			res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	}

	db = srv.pool.Get()
	defer srv.pool.Put(db)

	if item, err = db.ItemGetByID(id); err != nil {
		res.Message = fmt.Sprintf("Failed to lookup Item %d in database: %s",
			id,
			err.Error())
		srv.log.Printf("[ERROR] %s\n",
			res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	} else if item == nil {
		res.Message = fmt.Sprintf("Item %d does not exist in database", id)
		srv.log.Printf("[ERROR] %s\n",
			res.Message)
		hstatus = 400
		goto SEND_RESPONSE
	} else if err = db.ItemRate(item, int8(rating)); err != nil {
		res.Message = fmt.Sprintf("Failed to rate Item %q (%d): %s",
			item.Headline,
			item.ID,
			err.Error())
		srv.log.Printf("[ERROR] %s\n",
			res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	} else if err = srv.judge.Learn(item); err != nil {
		res.Message = fmt.Sprintf("Failed to train classifier on Item %q (%d): %s",
			item.Headline,
			item.ID,
			err.Error())
		srv.log.Printf("[ERROR] %s\n",
			res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	}

	res.Status = true
	res.Message = "Success"

SEND_RESPONSE:
	if sess != nil {
		if err = sess.Save(r, w); err != nil {
			srv.log.Printf("[ERROR] Failed to set session cookie: %s\n",
				err.Error())
		}
	}
	res.Timestamp = time.Now()
	if rbuf, err = json.Marshal(&res); err != nil {
		srv.log.Printf("[ERROR] Error serializing response: %s\n",
			err.Error())
		rbuf = errJSON(err.Error())
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.WriteHeader(hstatus)
	if _, err = w.Write(rbuf); err != nil {
		msg = fmt.Sprintf("Failed to send result: %s",
			err.Error())
		srv.log.Println("[ERROR] " + msg)
	}
} // func (srv *Server) handleAjaxRateItem(w http.ResponseWriter, r *http.Request)

func (srv *Server) handleAjaxUnrateItem(w http.ResponseWriter, r *http.Request) {
	srv.log.Printf("[TRACE] Handle request for %s from %s\n",
		r.URL.EscapedPath(),
		r.RemoteAddr)

	var (
		err     error
		sess    *sessions.Session
		rbuf    []byte
		db      *database.Database
		idstr   string
		id      int64
		item    *model.Item
		res     = Reply{Payload: make(map[string]string, 2)}
		msg     string
		vars    map[string]string
		hstatus = 200
	)

	vars = mux.Vars(r)

	if sess, err = srv.store.Get(r, sessionNameFrontend); err != nil {
		msg = fmt.Sprintf("Error getting client session from session store: %s",
			err.Error())
		srv.log.Println("[CRITICAL] " + msg)
		srv.sendErrorMessage(w, msg)
		return
	}

	idstr = vars["id"]

	if id, err = strconv.ParseInt(idstr, 10, 64); err != nil {
		res.Message = fmt.Sprintf("Cannot parse item ID %q: %s",
			idstr,
			err.Error())
		srv.log.Printf("[ERROR] %s\n",
			res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	}

	db = srv.pool.Get()
	defer srv.pool.Put(db)

	if item, err = db.ItemGetByID(id); err != nil {
		res.Message = fmt.Sprintf("Failed to lookup Item %d in database: %s",
			id,
			err.Error())
		srv.log.Printf("[ERROR] %s\n",
			res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	} else if item == nil {
		res.Message = fmt.Sprintf("Item %d does not exist in database", id)
		srv.log.Printf("[ERROR] %s\n",
			res.Message)
		hstatus = 400
		goto SEND_RESPONSE
	} else if err = db.ItemUnrate(item); err != nil {
		res.Message = fmt.Sprintf("Failed to rate Item %q (%d): %s",
			item.Headline,
			item.ID,
			err.Error())
		srv.log.Printf("[ERROR] %s\n",
			res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	}

	res.Status = true
	res.Message = "Success"
	res.Payload["cell"] = fmt.Sprintf(`
    <button type="button"
            class="btn btn-primary"
            onclick="rate_item(%d, 1);" >
      Interesting
    </button>
    <button type="button"
            class="btn btn-secondary"
            onclick="rate_item(%d, -1);" >
      Boring
    </button>
`,
		id, id)

SEND_RESPONSE:
	if sess != nil {
		if err = sess.Save(r, w); err != nil {
			srv.log.Printf("[ERROR] Failed to set session cookie: %s\n",
				err.Error())
		}
	}
	res.Timestamp = time.Now()
	if rbuf, err = json.Marshal(&res); err != nil {
		srv.log.Printf("[ERROR] Error serializing response: %s\n",
			err.Error())
		rbuf = errJSON(err.Error())
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.WriteHeader(hstatus)
	if _, err = w.Write(rbuf); err != nil {
		msg = fmt.Sprintf("Failed to send result: %s",
			err.Error())
		srv.log.Println("[ERROR] " + msg)
	}
} // func (srv *Server) handleAjaxUnrateItem(w http.ResponseWriter, r *http.Request)

func (srv *Server) handleAjaxTagView(w http.ResponseWriter, r *http.Request) {
	const tmplName = "tag_view"
	srv.log.Printf("[TRACE] Handle request for %s from %s\n",
		r.URL.EscapedPath(),
		r.RemoteAddr)
	var (
		err     error
		sess    *sessions.Session
		rbuf    []byte
		tbuf    bytes.Buffer
		db      *database.Database
		res     = Reply{Payload: make(map[string]string, 2)}
		tmpl    *template.Template
		hstatus = 200
		data    = tmplDataTagAll{
			tmplDataBase: tmplDataBase{
				Title: "All Tags",
				Debug: common.Debug,
				URL:   r.URL.EscapedPath(),
			},
		}
	)

	db = srv.pool.Get()
	defer srv.pool.Put(db)

	if sess, err = srv.store.Get(r, sessionNameFrontend); err != nil {
		res.Message = fmt.Sprintf("Error getting client session from session store: %s",
			err.Error())
		srv.log.Println("[CRITICAL] " + res.Message)
		srv.sendErrorMessage(w, res.Message)
		return
	} else if data.Tags, err = db.TagGetAll(); err != nil {
		res.Message = fmt.Sprintf("Failed to load all Tags: %s", err.Error())
		srv.log.Println("[CRITICAL] " + res.Message)
		srv.sendErrorMessage(w, res.Message)
		goto SEND_RESPONSE
	} else if data.ItemCnt, err = db.TagGetItemCnt(); err != nil {
		res.Message = fmt.Sprintf("Failed to get Item counts for Tags: %s",
			err.Error())
		srv.log.Println("[CRITICAL] " + res.Message)
		srv.sendErrorMessage(w, res.Message)
		goto SEND_RESPONSE
	} else if tmpl = srv.tmpl.Lookup(tmplName); tmpl == nil {
		res.Message = fmt.Sprintf("Failed to lookup template %s",
			tmplName)
		srv.log.Println("[CRITICAL] " + res.Message)
		srv.sendErrorMessage(w, res.Message)
		goto SEND_RESPONSE
	} else if err = tmpl.Execute(&tbuf, &data); err != nil {
		res.Message = fmt.Sprintf("Failed to render template %s: %s",
			tmplName,
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	}

	res.Payload["content"] = tbuf.String()
	res.Status = true

SEND_RESPONSE:
	if sess != nil {
		if err = sess.Save(r, w); err != nil {
			srv.log.Printf("[ERROR] Failed to set session cookie: %s\n",
				err.Error())
		}
	}
	res.Timestamp = time.Now()
	if rbuf, err = json.Marshal(&res); err != nil {
		srv.log.Printf("[ERROR] Error serializing response: %s\n",
			err.Error())
		rbuf = errJSON(err.Error())
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.WriteHeader(hstatus)
	if _, err = w.Write(rbuf); err != nil {
		var msg = fmt.Sprintf("Failed to send result: %s",
			err.Error())
		srv.log.Println("[ERROR] " + msg)
	}
} // func (srv *Server) handleAjaxTagView(w http.ResponseWriter, r *http.Request)

func (srv *Server) handleAjaxTagSubmit(w http.ResponseWriter, r *http.Request) {
	const tmplName = "tag_form"
	srv.log.Printf("[TRACE] Handle request for %s from %s\n",
		r.URL.EscapedPath(),
		r.RemoteAddr)
	var (
		err                    error
		sess                   *sessions.Session
		rbuf                   []byte
		tbuf                   bytes.Buffer
		db                     *database.Database
		res                    = Reply{Payload: make(map[string]string, 3)}
		msg, idstr, pstr, name string
		tagID, parentID        int64
		tag                    *model.Tag
		itemCnt                map[int64]int64
		tmpl                   *template.Template
		hstatus                = 200
		data                   = tmplDataTagForm{
			tmplDataBase: tmplDataBase{
				Title: "All Tags",
				Debug: common.Debug,
				URL:   r.URL.EscapedPath(),
			},
		}
	)

	if err = r.ParseForm(); err != nil {
		res.Message = fmt.Sprintf("Cannot parse form data: %s",
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 400
		goto SEND_RESPONSE
	}

	idstr = r.FormValue("id")
	name = r.FormValue("name")
	pstr = r.FormValue("parent")

	if pstr == "" {
		pstr = "0"
	}
	if idstr == "" {
		idstr = "0"
	}

	if sess, err = srv.store.Get(r, sessionNameFrontend); err != nil {
		res.Message = fmt.Sprintf("Error getting client session from session store: %s",
			err.Error())
		srv.log.Println("[CRITICAL] " + res.Message)
		srv.sendErrorMessage(w, res.Message)
		return
	} else if tagID, err = strconv.ParseInt(idstr, 10, 64); err != nil {
		res.Message = fmt.Sprintf("Cannot parse Tag ID %q: %s",
			idstr,
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 400
		goto SEND_RESPONSE
	} else if parentID, err = strconv.ParseInt(pstr, 10, 64); err != nil {
		res.Message = fmt.Sprintf("Cannot parse Parent ID %q: %s",
			pstr,
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 400
		goto SEND_RESPONSE
	} else if tmpl = srv.tmpl.Lookup(tmplName); tmpl == nil {
		res.Message = fmt.Sprintf("Failed to lookup template %s",
			tmplName)
		srv.log.Println("[CRITICAL] " + res.Message)
		srv.sendErrorMessage(w, res.Message)
		goto SEND_RESPONSE
	}

	db = srv.pool.Get()
	defer srv.pool.Put(db)

	defer func() {
		if res.Status {
			db.Commit() // nolint: errcheck
		} else {
			db.Rollback() // nolint: errcheck
		}
	}()

	if data.Tags, err = db.TagGetAll(); err != nil {
		res.Message = fmt.Sprintf("Failed to load all Tags: %s", err.Error())
		srv.log.Println("[CRITICAL] " + res.Message)
		srv.sendErrorMessage(w, res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	}

	// If the tag does not exist, we create it. If it does, we update it.
	// You know, we could use an UPSERT for that, couldn't we?
	// ... After looking at SQLite's UPSERT feature briefly, it looks like
	// this is not what I want.
	if tagID == 0 {
		if err = db.Begin(); err != nil {
			res.Message = fmt.Sprintf("Failed to start transaction for adding Tag: %s",
				err.Error())
			srv.log.Printf("[ERROR] %s\n", res.Message)
			hstatus = 500
			goto SEND_RESPONSE
		}

		tag = &model.Tag{
			Name:   name,
			Parent: parentID,
		}

		if err = db.TagAdd(tag); err != nil {
			res.Message = fmt.Sprintf("Error adding Tag %q to database: %s",
				name,
				err.Error())
			srv.log.Printf("[ERROR] %s\n", res.Message)
			hstatus = 500
			goto SEND_RESPONSE
		}

		data.Tag = *tag
	} else if tag, err = db.TagGetByID(tagID); err != nil {
		res.Message = fmt.Sprintf("Failed to load Tag %d: %s",
			tagID,
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	} else if tag == nil {
		res.Message = fmt.Sprintf("Tag %d was not found in database",
			tagID)
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	} else if err = db.TagUpdate(tag, name, parentID); err != nil {
		res.Message = fmt.Sprintf("Error updating Tag %s (%d): %s",
			tag.Name,
			tag.ID,
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	} else if itemCnt, err = db.TagGetItemCnt(); err != nil {
		res.Message = fmt.Sprintf("Failed to load Item Counts by Tag: %s",
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	}

	// Now render the updated form
	if err = tmpl.Execute(&tbuf, &data); err != nil {
		res.Message = fmt.Sprintf("Failed to render template %s: %s",
			tmplName,
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	} else if rbuf, err = json.Marshal(&data.Tag); err != nil {
		res.Message = fmt.Sprintf("Failed to serialize Tag %s (%d): %s",
			data.Tag.Name,
			data.Tag.ID,
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	}

	// res.Payload = map[string]string{
	// 	"content": tbuf.String(),
	// 	"tag":     string(rbuf),
	// 	"cnt":     strconv.FormatInt(itemCnt[data.Tag.ID], 10),
	// }
	res.Payload["content"] = tbuf.String()
	res.Payload["tag"] = string(rbuf)
	res.Payload["cnt"] = strconv.FormatInt(itemCnt[data.Tag.ID], 10)

	res.Status = true

SEND_RESPONSE:
	if sess != nil {
		if err = sess.Save(r, w); err != nil {
			srv.log.Printf("[ERROR] Failed to set session cookie: %s\n",
				err.Error())
		}
	}
	res.Timestamp = time.Now()
	if rbuf, err = json.Marshal(&res); err != nil {
		srv.log.Printf("[ERROR] Error serializing response: %s\n",
			err.Error())
		rbuf = errJSON(err.Error())
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.WriteHeader(hstatus)
	if _, err = w.Write(rbuf); err != nil {
		msg = fmt.Sprintf("Failed to send result: %s",
			err.Error())
		srv.log.Println("[ERROR] " + msg)
	}
} // func (srv *Server) handleAjaxTagSubmit(w http.ResponseWriter, r *http.Request)

// Ich Esel, ich sollte hier einfach das Formular neu rendern.
func (srv *Server) handleAjaxTagDetails(w http.ResponseWriter, r *http.Request) {
	srv.log.Printf("[TRACE] Handle request for %s from %s\n",
		r.URL.EscapedPath(),
		r.RemoteAddr)
	const tmplName = "tag_form"
	var (
		err   error
		sess  *sessions.Session
		rbuf  []byte
		tbuf  bytes.Buffer
		tmpl  *template.Template
		tag   *model.Tag
		idstr string
		id    int64
		db    *database.Database
		vars  map[string]string
		data  = tmplDataTagForm{
			tmplDataBase: tmplDataBase{
				Debug: common.Debug,
				URL:   r.URL.EscapedPath(),
			},
		}
		res = Reply{
			Payload: make(map[string]string, 2),
		}
		hstatus = 200
	)

	vars = mux.Vars(r)
	idstr = vars["id"]

	if id, err = strconv.ParseInt(idstr, 10, 64); err != nil {
		res.Message = fmt.Sprintf("Cannot parse Tag ID %q: %s",
			idstr,
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 400
		goto SEND_RESPONSE
	}

	db = srv.pool.Get()
	defer srv.pool.Put(db)

	if sess, err = srv.store.Get(r, sessionNameFrontend); err != nil {
		res.Message = fmt.Sprintf("Error getting client session from session store: %s",
			err.Error())
		srv.log.Println("[CRITICAL] " + res.Message)
		srv.sendErrorMessage(w, res.Message)
		return
	} else if tag, err = db.TagGetByID(id); err != nil {
		res.Message = fmt.Sprintf("Failed to load Tag %d: %s",
			id,
			err.Error())
		srv.log.Println("[CRITICAL] " + res.Message)
		srv.sendErrorMessage(w, res.Message)
		goto SEND_RESPONSE
	}

	data.Tag = *tag

	if data.Tags, err = db.TagGetAll(); err != nil {
		res.Message = fmt.Sprintf("Failed to load all Tags: %s",
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	} else if tmpl = srv.tmpl.Lookup(tmplName); tmpl == nil {
		res.Message = fmt.Sprintf("Did not find template %q", tmplName)
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	} else if err = tmpl.Execute(&tbuf, &data); err != nil {
		res.Message = fmt.Sprintf("Failed to render template %q: %s",
			tmplName,
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	}

	res.Payload["content"] = tbuf.String()
	res.Status = true

SEND_RESPONSE:
	if sess != nil {
		if err = sess.Save(r, w); err != nil {
			srv.log.Printf("[ERROR] Failed to set session cookie: %s\n",
				err.Error())
		}
	}
	res.Timestamp = time.Now()
	if rbuf, err = json.Marshal(&res); err != nil {
		srv.log.Printf("[ERROR] Error serializing response: %s\n",
			err.Error())
		rbuf = errJSON(err.Error())
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.WriteHeader(hstatus)
	if _, err = w.Write(rbuf); err != nil {
		var msg = fmt.Sprintf("Failed to send result: %s",
			err.Error())
		srv.log.Println("[ERROR] " + msg)
	}
} // func (srv *Server) handleAjaxTagDetails(w http.ResponseWriter, r *http.Request)

func (srv *Server) handleAjaxTagLinkAdd(w http.ResponseWriter, r *http.Request) {
	srv.log.Printf("[TRACE] Handle request for %s from %s\n",
		r.URL.EscapedPath(),
		r.RemoteAddr)
	var (
		err             error
		sess            *sessions.Session
		rbuf, pbuf      []byte
		tag             *model.Tag
		item            *model.Item
		istr, tstr, msg string
		tagID, itemID   int64
		db              *database.Database
		vars            map[string]string
		res             = Reply{
			Payload: make(map[string]string, 2),
		}
		hstatus = 200
	)

	vars = mux.Vars(r)
	istr = vars["item"]
	tstr = vars["tag"]

	if itemID, err = strconv.ParseInt(istr, 10, 64); err != nil {
		res.Message = fmt.Sprintf("Cannot parse Item ID %q: %s",
			istr,
			err.Error())
		srv.log.Printf("[CANTHAPPEN] %s\n", res.Message)
		hstatus = 400
		goto SEND_RESPONSE
	} else if tagID, err = strconv.ParseInt(tstr, 10, 64); err != nil {
		res.Message = fmt.Sprintf("Cannot parse Tag ID %q: %s",
			tstr,
			err.Error())
		srv.log.Printf("[CANTHAPPEN] %s\n", res.Message)
		hstatus = 400
		goto SEND_RESPONSE
	}

	db = srv.pool.Get()
	defer srv.pool.Put(db)

	if sess, err = srv.store.Get(r, sessionNameFrontend); err != nil {
		res.Message = fmt.Sprintf("Error getting client session from session store: %s",
			err.Error())
		srv.log.Println("[CRITICAL] " + res.Message)
		srv.sendErrorMessage(w, res.Message)
		return
	} else if tag, err = db.TagGetByID(tagID); err != nil {
		res.Message = fmt.Sprintf("Failed to load Tag %d: %s",
			tagID,
			err.Error())
		srv.log.Println("[CRITICAL] " + res.Message)
		srv.sendErrorMessage(w, res.Message)
		goto SEND_RESPONSE
	} else if tag == nil {
		res.Message = fmt.Sprintf("Did not find Tag %d in database", tagID)
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 400
		goto SEND_RESPONSE
	} else if item, err = db.ItemGetByID(itemID); err != nil {
		res.Message = fmt.Sprintf("Failed to load Item %d: %s",
			itemID, err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	} else if item == nil {
		res.Message = fmt.Sprintf("Did not find Item %d in database", itemID)
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 400
		goto SEND_RESPONSE
	} else if err = db.TagLinkAdd(item, tag); err != nil {
		res.Message = fmt.Sprintf("Failed to attach Tag %s (%d) to Item %d: %s",
			tag.Name,
			tag.ID,
			item.ID,
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	}

	if pbuf, err = json.Marshal(tag); err != nil {
		res.Message = fmt.Sprintf("Failed to serialize Tag %s (%d): %s",
			tag.Name,
			tag.ID,
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	}

	res.Payload["tag"] = string(pbuf)

	if pbuf, err = json.Marshal(item); err != nil {
		res.Message = fmt.Sprintf("Failed to serialize Item %d: %s",
			item.ID,
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	}

	res.Payload["item"] = string(pbuf)

	res.Status = true

SEND_RESPONSE:
	if sess != nil {
		if err = sess.Save(r, w); err != nil {
			srv.log.Printf("[ERROR] Failed to set session cookie: %s\n",
				err.Error())
		}
	}
	res.Timestamp = time.Now()
	if rbuf, err = json.Marshal(&res); err != nil {
		srv.log.Printf("[ERROR] Error serializing response: %s\n",
			err.Error())
		rbuf = errJSON(err.Error())
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.WriteHeader(hstatus)
	if _, err = w.Write(rbuf); err != nil {
		msg = fmt.Sprintf("Failed to send result: %s",
			err.Error())
		srv.log.Println("[ERROR] " + msg)
	}
} // func (srv *Server) handleAjaxTagLinkAdd(w http.ResponseWriter, r *http.Request)

func (srv *Server) handleAjaxTagLinkRemove(w http.ResponseWriter, r *http.Request) {
	srv.log.Printf("[TRACE] Handle request for %s from %s\n",
		r.URL.EscapedPath(),
		r.RemoteAddr)
	var (
		err             error
		sess            *sessions.Session
		rbuf            []byte
		tag             *model.Tag
		item            *model.Item
		istr, tstr, msg string
		tagID, itemID   int64
		db              *database.Database
		vars            map[string]string
		res             = Reply{
			Payload: make(map[string]string, 2),
		}
		hstatus = 200
	)

	vars = mux.Vars(r)
	istr = vars["item"]
	tstr = vars["tag"]

	if itemID, err = strconv.ParseInt(istr, 10, 64); err != nil {
		res.Message = fmt.Sprintf("Cannot parse Item ID %q: %s",
			istr,
			err.Error())
		srv.log.Printf("[CANTHAPPEN] %s\n", res.Message)
		hstatus = 400
		goto SEND_RESPONSE
	} else if tagID, err = strconv.ParseInt(tstr, 10, 64); err != nil {
		res.Message = fmt.Sprintf("Cannot parse Tag ID %q: %s",
			tstr,
			err.Error())
		srv.log.Printf("[CANTHAPPEN] %s\n", res.Message)
		hstatus = 400
		goto SEND_RESPONSE
	}

	db = srv.pool.Get()
	defer srv.pool.Put(db)

	if sess, err = srv.store.Get(r, sessionNameFrontend); err != nil {
		res.Message = fmt.Sprintf("Error getting client session from session store: %s",
			err.Error())
		srv.log.Println("[CRITICAL] " + res.Message)
		srv.sendErrorMessage(w, res.Message)
		return
	} else if tag, err = db.TagGetByID(tagID); err != nil {
		res.Message = fmt.Sprintf("Failed to load Tag %d: %s",
			tagID,
			err.Error())
		srv.log.Println("[CRITICAL] " + res.Message)
		srv.sendErrorMessage(w, res.Message)
		goto SEND_RESPONSE
	} else if tag == nil {
		res.Message = fmt.Sprintf("Did not find Tag %d in database", tagID)
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 400
		goto SEND_RESPONSE
	} else if item, err = db.ItemGetByID(itemID); err != nil {
		res.Message = fmt.Sprintf("Failed to load Item %d: %s",
			itemID, err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	} else if item == nil {
		res.Message = fmt.Sprintf("Did not find Item %d in database", itemID)
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 400
		goto SEND_RESPONSE
	} else if err = db.TagLinkDelete(item, tag); err != nil {
		res.Message = fmt.Sprintf("Failed to remove link of Tag %s (%d) to Item %d: %s",
			tag.Name,
			tag.ID,
			item.ID,
			err.Error())
		srv.log.Printf("[ERROR] %s\n", res.Message)
		hstatus = 500
		goto SEND_RESPONSE
	}

	res.Status = true
	res.Message = fmt.Sprintf("Link Tag(%d)->Item(%d) was successfully removed",
		tag.ID,
		item.ID)

SEND_RESPONSE:
	if sess != nil {
		if err = sess.Save(r, w); err != nil {
			srv.log.Printf("[ERROR] Failed to set session cookie: %s\n",
				err.Error())
		}
	}
	res.Timestamp = time.Now()
	if rbuf, err = json.Marshal(&res); err != nil {
		srv.log.Printf("[ERROR] Error serializing response: %s\n",
			err.Error())
		rbuf = errJSON(err.Error())
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	w.WriteHeader(hstatus)
	if _, err = w.Write(rbuf); err != nil {
		msg = fmt.Sprintf("Failed to send result: %s",
			err.Error())
		srv.log.Println("[ERROR] " + msg)
	}
} // func (srv *Server) handleAjaxTagLinkRemove(w http.ResponseWriter, r *http.Request)
