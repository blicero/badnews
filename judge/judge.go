// /home/krylon/go/src/github.com/blicero/badnews/judge/judge.go
// -*- mode: go; coding: utf-8; -*-
// Created on 04. 10. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-11-02 20:13:19 krylon>

// Package judge provides the guessing of ratings for items that have not been manually rated.
package judge

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	bt "go.etcd.io/bbolt" // Use the BoltDB backend

	"github.com/blicero/badnews/common"
	"github.com/blicero/badnews/common/path"
	"github.com/blicero/badnews/database"
	"github.com/blicero/badnews/logdomain"
	"github.com/blicero/badnews/model"
	"github.com/blicero/shield"
	"github.com/endeveit/guesslanguage"
	"github.com/faabiosr/cachego"
	"github.com/faabiosr/cachego/bolt"
)

// TODO I'll increase this value once I'm done testing.
const cacheTimeout = time.Minute * 240

// Judge is a classifier to rate News Items as boring or interesting.
type Judge struct {
	log   *log.Logger
	jdg   map[string]shield.Shield
	db    *database.Database
	bdb   *bt.DB
	cache cachego.Cache
	lock  sync.RWMutex
}

// New creates a new Judge
func New() (*Judge, error) {
	var (
		err error
		j   = &Judge{
			jdg: map[string]shield.Shield{
				"de": shield.New(
					shield.NewGermanTokenizer(),
					shield.NewLevelDBStore(filepath.Join(
						common.Path(path.Judge),
						"de")),
				),
				"en": shield.New(
					shield.NewEnglishTokenizer(),
					shield.NewLevelDBStore(
						filepath.Join(
							common.Path(path.Judge),
							"en",
						),
					),
				),
			},
		}
	)

	if j.log, err = common.GetLogger(logdomain.Judge); err != nil {
		return nil, err
	} else if j.db, err = database.Open(common.Path(path.Database)); err != nil {
		return nil, err
	} else if j.bdb, err = bt.Open(common.Path(path.JudgeCache), 0600, nil); err != nil {
		j.log.Printf("[CRITICAL] Cannot open jcache db at %s: %s\n",
			common.Path(path.JudgeCache),
			err.Error())
		j.db.Close() // nolint: errcheck
		return nil, err
	}

	j.cache = bolt.New(j.bdb)

	return j, nil
} // func New() (*Judge, error)

func (j *Judge) Rate(i *model.Item) (string, error) {
	var (
		err                error
		rating, lang, body string
		s                  shield.Shield
	)

	j.lock.RLock()
	defer j.lock.RUnlock()

	if rating, err = j.cache.Fetch(i.IDString()); err != nil {
		if strings.Contains(err.Error(), "cache expired") {
			// tough luck
		} else {
			j.log.Printf("[ERROR] Failed to lookup Item %q (%d) in cache: %s\n",
				i.Headline,
				i.ID,
				err.Error())
		}
	} else if rating != "" {
		return rating, nil
	}

	lang, body = j.getLanguage(i)

	if s = j.jdg[lang]; s == nil {
		s = j.jdg["en"]
	}

	if rating, err = s.Classify(body); err != nil {
		return "", err
	}

	switch rating {
	case "interesting":
		i.Guessed = 1
	case "boring":
		i.Guessed = -1
	case "unknown":
		// yeah, no
	default:
		j.log.Printf("[CANTHAPPEN] Unexpected rating from Judge: %q\n",
			rating)
	}

	if err = j.cache.Save(i.IDString(), rating, cacheTimeout); err != nil {
		j.log.Printf("[ERROR] Failed to save rating for Item %q (%d) in cache: %s\n",
			i.Headline,
			i.ID,
			err.Error())
	}

	return rating, nil
} // func (j *Judge) Rate(i *model.Item) (string, error)

// Reset discards the existing training data.
func (j *Judge) Reset() error {
	var err error

	j.lock.Lock()
	defer j.lock.Unlock()

	for k, v := range j.jdg {
		if err = v.Reset(); err != nil {
			j.log.Printf("[ERROR] Failed to reset Judge for %s: %s\n",
				k,
				err.Error())
			return err
		}
	}

	return nil
} // func (j *Judge) Reset() error

// Train trains the Judge.
func (j *Judge) Train() error {
	var (
		err   error
		items []model.Item
	)

	// j.lock.Lock()
	// defer j.lock.Unlock()

	if items, err = j.db.ItemGetRated(); err != nil {
		j.log.Printf("[ERROR] Cannot load rated Items: %s\n", err.Error())
		return err
	}

	j.log.Printf("[DEBUG] Training classifier on %d items\n", len(items))

	for _, i := range items {
		if err = j.Learn(&i); err != nil {
			j.log.Printf("[ERROR] Cannot train on Item %q (%d): %s\n",
				i.Headline,
				i.ID,
				err.Error())
			return err
		}
	}

	return nil
} // func (j *Judge) Train() error

// Learn adds a single item to the Judge's training corpus.
func (j *Judge) Learn(i *model.Item) error {
	var (
		err               error
		lng, body, bucket string
		s                 shield.Shield
	)

	j.lock.Lock()
	defer j.lock.Unlock()

	switch i.Rating {
	case -1:
		bucket = "boring"
	case 1:
		bucket = "interesting"
	default:
		return fmt.Errorf("Invalid rating for Item %q (%d): %d",
			i.Headline,
			i.ID,
			i.Rating)
	}

	lng, body = j.getLanguage(i)

	if s = j.jdg[lng]; s == nil {
		s = j.jdg["en"]
	}

	if err = s.Learn(bucket, body); err != nil {
		j.log.Printf("[ERROR] Failed to learn Item %d (%q): %s\n",
			i.ID,
			i.Headline,
			err.Error())
		return err
	}

	return nil
} // func (j *Judge) Learn(t *tag.Tag, i *feed.Item) error

// Unlearn makes the Judge forget about an Item.
func (j *Judge) Unlearn(i *model.Item) error {
	var (
		err               error
		lng, body, bucket string
		s                 shield.Shield
	)

	j.lock.Lock()
	defer j.lock.Unlock()

	switch i.Rating {
	case -1:
		bucket = "boring"
	case 1:
		bucket = "interesting"
	default:
		return fmt.Errorf("Invalid rating for Item %q (%d): %d",
			i.Headline,
			i.ID,
			i.Rating)
	}

	lng, body = j.getLanguage(i)

	if s = j.jdg[lng]; s == nil {
		s = j.jdg["en"]
	}

	if err = s.Forget(bucket, body); err != nil {
		j.log.Printf("[ERROR] Failed to learn Item %d (%q): %s\n",
			i.ID,
			i.Headline,
			err.Error())
		return err
	}

	return nil
} // func (j *Judge) Unlearn(t *tag.Tag, i *feed.Item) error

func (j *Judge) getLanguage(item *model.Item) (lng, fullText string) {
	const (
		defaultLang = "en"
	)

	var (
		err        error
		lang, body string
		blString   = []string{
			"Lauren Boebert buried in ridicule after claim about 1930s Germany",
			"GOP's Madison Cawthorn ruthlessly mocked for wailing about 'scary' proof of vaccination",
		}
	)

	body = item.Plaintext()

	defer func() {
		if x := recover(); x != nil {
			var m bool
			for _, bl := range blString {
				if strings.Contains(item.Headline, bl) {
					m = true
					break
				}
			}
			if !m {
				var buf [2048]byte
				var cnt = runtime.Stack(buf[:], false)
				j.log.Printf("[CRITICAL] Panic in getLanguage for Item %q: %s\n%s",
					item.Headline,
					x,
					string(buf[:cnt]))
			}
			lng = defaultLang
			fullText = body
		}
	}()

	if lang, err = guesslanguage.Guess(body); err != nil {
		j.log.Printf("[ERROR] Cannot determine language of Item %q: %s\n",
			item.Headline,
			err.Error())
		lang = defaultLang
	}

	return lang, body
} // func getLanguage(title, description string) (string, string)
