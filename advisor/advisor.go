// /home/krylon/go/src/ticker/tag/advisor.go
// -*- mode: go; coding: utf-8; -*-
// Created on 10. 03. 2021 by Benjamin Walkenhorst
// (c) 2021 Benjamin Walkenhorst
// Time-stamp: <2024-10-28 20:42:55 krylon>

// Package advisor provides suggestions on what Tags one might want to attach
// to news Items.
package advisor

import (
	"log"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/blicero/badnews/common"
	"github.com/blicero/badnews/common/path"
	"github.com/blicero/badnews/database"
	"github.com/blicero/badnews/logdomain"
	"github.com/blicero/badnews/model"
	"github.com/blicero/shield"

	"github.com/blicero/krylib"
	"github.com/endeveit/guesslanguage"
)

// var nonword = regexp.MustCompile(`\W+`)

// SuggestedTag is a suggestion to attach a specific Tag to a specific Item.
type SuggestedTag struct {
	model.Tag
	Score float64
}

// Advisor can suggest Tags for News Items.
type Advisor struct {
	db     *database.Database
	log    *log.Logger
	shield map[string]shield.Shield
	tags   map[string]*model.Tag
}

// NewAdvisor returns a new Advisor, but it does not train it, yet.
func NewAdvisor() (*Advisor, error) {
	var (
		err error
		adv = &Advisor{
			shield: map[string]shield.Shield{
				"de": shield.New(
					shield.NewGermanTokenizer(),
					shield.NewLevelDBStore(filepath.Join(
						common.Path(path.Advisor),
						"de")),
				),
				"en": shield.New(
					shield.NewEnglishTokenizer(),
					shield.NewLevelDBStore(
						filepath.Join(
							common.Path(path.Advisor),
							"en",
						),
					),
				),
			},
		}
	)

	if adv.log, err = common.GetLogger(logdomain.Advisor); err != nil {
		return nil, err
	} else if adv.db, err = database.Open(common.Path(path.Database)); err != nil {
		adv.log.Printf("[ERROR] Cannot open database: %s\n",
			err.Error())
		return nil, err
	} else if err = adv.loadTags(); err != nil {
		return nil, err
	}

	return adv, nil
} // func NewAdvisor() (*Advisor, error)

func (adv *Advisor) loadTags() error {
	var (
		err  error
		tags []*model.Tag
	)

	if tags, err = adv.db.TagGetAll(); err != nil {
		adv.log.Printf("[ERROR] Cannot load all Tags from database: %s\n",
			err.Error())
		return err
	}

	adv.tags = make(map[string]*model.Tag, len(tags))

	for _, t := range tags {
		adv.tags[t.Name] = t
	}

	return nil
} // func (adv *advisor) loadTags() error

// Train trains the Advisor based on the Tags that have been attached to
// Items previously.
func (adv *Advisor) Train() error {
	var (
		err   error
		tags  []*model.Tag
		items []*model.Item
	)

	for k, v := range adv.shield {
		adv.log.Printf("[DEBUG] Reset Shield instance for %s\n",
			k)
		if err = v.Reset(); err != nil {
			adv.log.Printf("[ERROR] Cannot reset Shield/%s: %s\n",
				k,
				err.Error())
			return err
		}
	}

	if tags, err = adv.db.TagGetAll(); err != nil {
		adv.log.Printf("·[ERROR] Failed to load all tags: %s\n",
			err.Error())
		return err
	}

	for _, t := range tags {
		if items, err = adv.db.TagLinkGetByTag(t); err != nil {
			adv.log.Printf("[ERROR] Failed to load Items for Tag %s: %s",
				t.Name,
				err.Error())
			return err
		}

		for _, item := range items {
			var (
				lng, body string
				s         shield.Shield
			)

			lng, body = adv.getLanguage(item)

			if s = adv.shield[lng]; s == nil {
				s = adv.shield["en"]
			}

			if err = s.Learn(t.Name, body); err != nil {
				adv.log.Printf("[ERROR] Failed to learn Item %d (%q): %s\n",
					item.ID,
					item.Headline,
					err.Error())
				return err
			}
		}
	}

	return nil
} // func (adv *Advisor) Train() error

// Learn adds a single item to the Advisor's training corpus.
func (adv *Advisor) Learn(t *model.Tag, i *model.Item) error {
	var (
		err       error
		lng, body string
		s         shield.Shield
	)

	lng, body = adv.getLanguage(i)

	if s = adv.shield[lng]; s == nil {
		s = adv.shield["en"]
	}

	if err = s.Learn(t.Name, body); err != nil {
		adv.log.Printf("[ERROR] Failed to learn Item %d (%q): %s\n",
			i.ID,
			i.Headline,
			err.Error())
		return err
	}

	return nil
} // func (adv *Advisor) Learn(t *model.Tag, i *model.Item) error

// Unlearn removes the association between an Item and a Tag from the Advisor corpus.
func (adv *Advisor) Unlearn(t *model.Tag, i *model.Item) error {
	var (
		err       error
		lng, body string
		s         shield.Shield
	)

	lng, body = adv.getLanguage(i)

	if s = adv.shield[lng]; s == nil {
		s = adv.shield["en"]
	}

	if err = s.Forget(t.Name, body); err != nil {
		adv.log.Printf("[ERROR] Failed to learn Item %d (%q): %s\n",
			i.ID,
			i.Headline,
			err.Error())
		return err
	}

	return nil
} // func (adv *Advisor) Unlearn(t *model.Tag, i *model.Item) error

type suggList []SuggestedTag

func (s suggList) Len() int           { return len(s) }
func (s suggList) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s suggList) Less(i, j int) bool { return s[j].Score < s[i].Score }

// Suggest returns a map Tags and how likely they apply to the given Item.
func (adv *Advisor) Suggest(item *model.Item, n int) []SuggestedTag {
	var (
		err        error
		res        map[string]float64
		lang, body string
		s          shield.Shield
	)

	lang, body = adv.getLanguage(item)

	if s = adv.shield[lang]; s == nil {
		s = adv.shield["en"]
	}

	if res, err = s.Score(body); err != nil {
		adv.log.Printf("[ERROR] Failed to Score Item %d (%q): %s\n",
			item.ID,
			item.Headline,
			err.Error())
		return nil
	}

	var list = make(suggList, 0, len(res))

	for c, r := range res {
		if c == "unknown" {
			continue
		} else if t, ok := adv.tags[c]; ok {
			if !item.HasTag(t.ID) {
				var s = SuggestedTag{Tag: *t, Score: r * 100}
				list = append(list, s)
			}
		} else {
			adv.log.Printf("[CRITICAL] Invalid tag suggested for Item %q (%d):\n%#v\n",
				item.Headline,
				item.ID,
				res)
		}
	}

	var cnt = krylib.Min(len(list), n)
	sort.Sort(list)

	return list[:cnt]
} // func (adv *Advisor) Suggest(item *model.Item) []SuggestedTag

func (adv *Advisor) getLanguage(item *model.Item) (lng, fullText string) {
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
				adv.log.Printf("[CRITICAL] Panic in getLanguage for Item %q: %s\n%s",
					item.Headline,
					x,
					string(buf[:cnt]))
			}
			lng = defaultLang
			fullText = body
		}
	}()

	if lang, err = guesslanguage.Guess(body); err != nil {
		adv.log.Printf("[ERROR] Cannot determine language of Item %q: %s\n",
			item.Headline,
			err.Error())
		lang = defaultLang
	}

	return lang, body
} // func getLanguage(title, description string) (string, string)
