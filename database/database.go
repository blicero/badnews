// /home/krylon/go/src/github.com/blicero/badnews/database/database.go
// -*- mode: go; coding: utf-8; -*-
// Created on 19. 09. 2024 by Benjamin Walkenhorst
// (c) 2024 Benjamin Walkenhorst
// Time-stamp: <2024-12-03 18:31:04 krylon>

// Package database provides persistence.
package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blicero/badnews/common"
	"github.com/blicero/badnews/database/query"
	"github.com/blicero/badnews/logdomain"
	"github.com/blicero/badnews/model"
	"github.com/blicero/krylib"
	_ "github.com/mattn/go-sqlite3" // Import the database driver
)

var (
	openLock sync.Mutex
	idCnt    int64
)

// ErrTxInProgress indicates that an attempt to initiate a transaction failed
// because there is already one in progress.
var ErrTxInProgress = errors.New("A Transaction is already in progress")

// ErrNoTxInProgress indicates that an attempt was made to finish a
// transaction when none was active.
var ErrNoTxInProgress = errors.New("There is no transaction in progress")

// ErrEmptyUpdate indicates that an update operation would not change any
// values.
var ErrEmptyUpdate = errors.New("Update operation does not change any values")

// ErrInvalidValue indicates that one or more parameters passed to a method
// had values that are invalid for that operation.
var ErrInvalidValue = errors.New("Invalid value for parameter")

// ErrObjectNotFound indicates that an Object was not found in the database.
var ErrObjectNotFound = errors.New("object was not found in database")

// ErrInvalidSavepoint is returned when a user of the Database uses an unkown
// (or expired) savepoint name.
var ErrInvalidSavepoint = errors.New("that save point does not exist")

// If a query returns an error and the error text is matched by this regex, we
// consider the error as transient and try again after a short delay.
var retryPat = regexp.MustCompile("(?i)database is (?:locked|busy)")

// worthARetry returns true if an error returned from the database
// is matched by the retryPat regex.
func worthARetry(e error) bool {
	return retryPat.MatchString(e.Error())
} // func worthARetry(e error) bool

// retryDelay is the amount of time we wait before we repeat a database
// operation that failed due to a transient error.
const retryDelay = 25 * time.Millisecond

func waitForRetry() {
	time.Sleep(retryDelay)
} // func waitForRetry()

// Database wraps a database connection and associated state.
type Database struct {
	id            int64
	db            *sql.DB
	tx            *sql.Tx
	log           *log.Logger
	path          string
	spNameCounter int
	spNameCache   map[string]string
	queries       map[query.ID]*sql.Stmt
}

// Open opens a Database. If the database specified by the path does not exist,
// yet, it is created and initialized.
func Open(path string) (*Database, error) {
	var (
		err      error
		dbExists bool
		db       = &Database{
			path:          path,
			spNameCounter: 1,
			spNameCache:   make(map[string]string),
			queries:       make(map[query.ID]*sql.Stmt),
		}
	)

	openLock.Lock()
	defer openLock.Unlock()
	idCnt++
	db.id = idCnt

	if db.log, err = common.GetLogger(logdomain.Database); err != nil {
		return nil, err
	} else if common.Debug {
		db.log.Printf("[DEBUG] Open database %s\n", path)
	}

	var connstring = fmt.Sprintf("%s?_locking=NORMAL&_journal=WAL&_fk=true&recursive_triggers=true",
		path)

	if dbExists, err = krylib.Fexists(path); err != nil {
		db.log.Printf("[ERROR] Failed to check if %s already exists: %s\n",
			path,
			err.Error())
		return nil, err
	} else if db.db, err = sql.Open("sqlite3", connstring); err != nil {
		db.log.Printf("[ERROR] Failed to open %s: %s\n",
			path,
			err.Error())
		return nil, err
	}

	if !dbExists {
		if err = db.initialize(); err != nil {
			var e2 error
			if e2 = db.db.Close(); e2 != nil {
				db.log.Printf("[CRITICAL] Failed to close database: %s\n",
					e2.Error())
				return nil, e2
			} else if e2 = os.Remove(path); e2 != nil {
				db.log.Printf("[CRITICAL] Failed to remove database file %s: %s\n",
					db.path,
					e2.Error())
			}
			return nil, err
		}
		db.log.Printf("[INFO] Database at %s has been initialized\n",
			path)
	}

	return db, nil
} // func Open(path string) (*Database, error)

func (db *Database) initialize() error {
	var err error
	var tx *sql.Tx

	if common.Debug {
		db.log.Printf("[DEBUG] Initialize fresh database at %s\n",
			db.path)
	}

	if tx, err = db.db.Begin(); err != nil {
		db.log.Printf("[ERROR] Cannot begin transaction: %s\n",
			err.Error())
		return err
	}

	for _, q := range initQueries {
		db.log.Printf("[TRACE] Execute init query:\n%s\n",
			q)
		if _, err = tx.Exec(q); err != nil {
			db.log.Printf("[ERROR] Cannot execute init query: %s\n%s\n",
				err.Error(),
				q)
			if rbErr := tx.Rollback(); rbErr != nil {
				db.log.Printf("[CANTHAPPEN] Cannot rollback transaction: %s\n",
					rbErr.Error())
				return rbErr
			}
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		db.log.Printf("[CANTHAPPEN] Failed to commit init transaction: %s\n",
			err.Error())
		return err
	}

	return nil
} // func (db *Database) initialize() error

// Close closes the database.
// If there is a pending transaction, it is rolled back.
func (db *Database) Close() error {
	// I wonder if would make more snese to panic() if something goes wrong

	var err error

	if db.tx != nil {
		if err = db.tx.Rollback(); err != nil {
			db.log.Printf("[CRITICAL] Cannot roll back pending transaction: %s\n",
				err.Error())
			return err
		}
		db.tx = nil
	}

	for key, stmt := range db.queries {
		if err = stmt.Close(); err != nil {
			db.log.Printf("[CRITICAL] Cannot close statement handle %s: %s\n",
				key,
				err.Error())
			return err
		}
		delete(db.queries, key)
	}

	if err = db.db.Close(); err != nil {
		db.log.Printf("[CRITICAL] Cannot close database: %s\n",
			err.Error())
	}

	db.db = nil
	return nil
} // func (db *Database) Close() error

func (db *Database) getQuery(id query.ID) (*sql.Stmt, error) {
	var (
		stmt  *sql.Stmt
		found bool
		err   error
	)

	if stmt, found = db.queries[id]; found {
		return stmt, nil
	} else if _, found = dbQueries[id]; !found {
		return nil, fmt.Errorf("Unknown Query %d",
			id)
	}

	db.log.Printf("[TRACE] Prepare query %s\n", id)

PREPARE_QUERY:
	if stmt, err = db.db.Prepare(dbQueries[id]); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto PREPARE_QUERY
		}

		db.log.Printf("[ERROR] Cannot parse query %s: %s\n%s\n",
			id,
			err.Error(),
			dbQueries[id])
		return nil, err
	}

	db.queries[id] = stmt
	return stmt, nil
} // func (db *Database) getQuery(query.ID) (*sql.Stmt, error)

func (db *Database) resetSPNamespace() {
	db.spNameCounter = 1
	db.spNameCache = make(map[string]string)
} // func (db *Database) resetSPNamespace()

func (db *Database) generateSPName(name string) string {
	var spname = fmt.Sprintf("Savepoint%05d",
		db.spNameCounter)

	db.spNameCache[name] = spname
	db.spNameCounter++
	return spname
} // func (db *Database) generateSPName() string

// PerformMaintenance performs some maintenance operations on the database.
// It cannot be called while a transaction is in progress and will block
// pretty much all access to the database while it is running.
func (db *Database) PerformMaintenance() error {
	var mQueries = []string{
		"PRAGMA wal_checkpoint(TRUNCATE)",
		"VACUUM",
		"REINDEX",
		"ANALYZE",
	}
	var err error

	if db.tx != nil {
		return ErrTxInProgress
	}

	for _, q := range mQueries {
		if _, err = db.db.Exec(q); err != nil {
			db.log.Printf("[ERROR] Failed to execute %s: %s\n",
				q,
				err.Error())
		}
	}

	return nil
} // func (db *Database) PerformMaintenance() error

// Begin begins an explicit database transaction.
// Only one transaction can be in progress at once, attempting to start one,
// while another transaction is already in progress will yield ErrTxInProgress.
func (db *Database) Begin() error {
	var err error

	db.log.Printf("[DEBUG] Database#%d Begin Transaction\n",
		db.id)

	if db.tx != nil {
		return ErrTxInProgress
	}

BEGIN_TX:
	for db.tx == nil {
		if db.tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				continue BEGIN_TX
			} else {
				db.log.Printf("[ERROR] Failed to start transaction: %s\n",
					err.Error())
				return err
			}
		}
	}

	db.resetSPNamespace()

	return nil
} // func (db *Database) Begin() error

// SavepointCreate creates a savepoint with the given name.
//
// Savepoints only make sense within a running transaction, and just like
// with explicit transactions, managing them is the responsibility of the
// user of the Database.
//
// Creating a savepoint without a surrounding transaction is not allowed,
// even though SQLite allows it.
//
// For details on how Savepoints work, check the excellent SQLite
// documentation, but here's a quick guide:
//
// Savepoints are kind-of-like transactions within a transaction: One
// can create a savepoint, make some changes to the database, and roll
// back to that savepoint, discarding all changes made between
// creating the savepoint and rolling back to it. Savepoints can be
// quite useful, but there are a few things to keep in mind:
//
//   - Savepoints exist within a transaction. When the surrounding transaction
//     is finished, all savepoints created within that transaction cease to exist,
//     no matter if the transaction is commited or rolled back.
//
//   - When the database is recovered after being interrupted during a
//     transaction, e.g. by a power outage, the entire transaction is rolled back,
//     including all savepoints that might exist.
//
//   - When a savepoint is released, nothing changes in the state of the
//     surrounding transaction. That means rolling back the surrounding
//     transaction rolls back the entire transaction, regardless of any
//     savepoints within.
//
//   - Savepoints do not nest. Releasing a savepoint releases it and *all*
//     existing savepoints that have been created before it. Rolling back to a
//     savepoint removes that savepoint and all savepoints created after it.
func (db *Database) SavepointCreate(name string) error {
	var err error

	db.log.Printf("[DEBUG] SavepointCreate(%s)\n",
		name)

	if db.tx == nil {
		return ErrNoTxInProgress
	}

SAVEPOINT:
	// It appears that the SAVEPOINT statement does not support placeholders.
	// But I do want to used named savepoints.
	// And I do want to use the given name so that no SQL injection
	// becomes possible.
	// It would be nice if the database package or at least the SQLite
	// driver offered a way to escape the string properly.
	// One possible solution would be to use names generated by the
	// Database instead of user-defined names.
	//
	// But then I need a way to use the Database-generated name
	// in rolling back and releasing the savepoint.
	// I *could* use the names strictly inside the Database, store them in
	// a map or something and hand out a key to that name to the user.
	// Since savepoint only exist within one transaction, I could even
	// re-use names from one transaction to the next.
	//
	// Ha! I could accept arbitrary names from the user, generate a
	// clean name, and store these in a map. That way the user can
	// still choose names that are outwardly visible, but they do
	// not touch the Database itself.
	//
	//if _, err = db.tx.Exec("SAVEPOINT ?", name); err != nil {
	// if _, err = db.tx.Exec("SAVEPOINT " + name); err != nil {
	// 	if worthARetry(err) {
	// 		waitForRetry()
	// 		goto SAVEPOINT
	// 	}

	// 	db.log.Printf("[ERROR] Failed to create savepoint %s: %s\n",
	// 		name,
	// 		err.Error())
	// }

	var internalName = db.generateSPName(name)

	var spQuery = "SAVEPOINT " + internalName

	if _, err = db.tx.Exec(spQuery); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto SAVEPOINT
		}

		db.log.Printf("[ERROR] Failed to create savepoint %s: %s\n",
			name,
			err.Error())
	}

	return err
} // func (db *Database) SavepointCreate(name string) error

// SavepointRelease releases the Savepoint with the given name, and all
// Savepoints created before the one being release.
func (db *Database) SavepointRelease(name string) error {
	var (
		err                   error
		internalName, spQuery string
		validName             bool
	)

	db.log.Printf("[DEBUG] SavepointRelease(%s)\n",
		name)

	if db.tx != nil {
		return ErrNoTxInProgress
	}

	if internalName, validName = db.spNameCache[name]; !validName {
		db.log.Printf("[ERROR] Attempt to release unknown Savepoint %q\n",
			name)
		return ErrInvalidSavepoint
	}

	db.log.Printf("[DEBUG] Release Savepoint %q (%q)",
		name,
		db.spNameCache[name])

	spQuery = "RELEASE SAVEPOINT " + internalName

SAVEPOINT:
	if _, err = db.tx.Exec(spQuery); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto SAVEPOINT
		}

		db.log.Printf("[ERROR] Failed to release savepoint %s: %s\n",
			name,
			err.Error())
	} else {
		delete(db.spNameCache, internalName)
	}

	return err
} // func (db *Database) SavepointRelease(name string) error

// SavepointRollback rolls back the running transaction to the given savepoint.
func (db *Database) SavepointRollback(name string) error {
	var (
		err                   error
		internalName, spQuery string
		validName             bool
	)

	db.log.Printf("[DEBUG] SavepointRollback(%s)\n",
		name)

	if db.tx != nil {
		return ErrNoTxInProgress
	}

	if internalName, validName = db.spNameCache[name]; !validName {
		return ErrInvalidSavepoint
	}

	spQuery = "ROLLBACK TO SAVEPOINT " + internalName

SAVEPOINT:
	if _, err = db.tx.Exec(spQuery); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto SAVEPOINT
		}

		db.log.Printf("[ERROR] Failed to create savepoint %s: %s\n",
			name,
			err.Error())
	}

	delete(db.spNameCache, name)
	return err
} // func (db *Database) SavepointRollback(name string) error

// Rollback terminates a pending transaction, undoing any changes to the
// database made during that transaction.
// If no transaction is active, it returns ErrNoTxInProgress
func (db *Database) Rollback() error {
	var err error

	db.log.Printf("[DEBUG] Database#%d Roll back Transaction\n",
		db.id)

	if db.tx == nil {
		return ErrNoTxInProgress
	} else if err = db.tx.Rollback(); err != nil {
		return fmt.Errorf("Cannot roll back database transaction: %s",
			err.Error())
	}

	db.tx = nil
	db.resetSPNamespace()

	return nil
} // func (db *Database) Rollback() error

// Commit ends the active transaction, making any changes made during that
// transaction permanent and visible to other connections.
// If no transaction is active, it returns ErrNoTxInProgress
func (db *Database) Commit() error {
	var err error

	db.log.Printf("[DEBUG] Database#%d Commit Transaction\n",
		db.id)

	if db.tx == nil {
		return ErrNoTxInProgress
	} else if err = db.tx.Commit(); err != nil {
		return fmt.Errorf("Cannot commit transaction: %s",
			err.Error())
	}

	db.resetSPNamespace()
	db.tx = nil
	return nil
} // func (db *Database) Commit() error

// FeedAdd enters a Feed into the database.
func (db *Database) FeedAdd(f *model.Feed) error {
	const qid query.ID = query.FeedAdd
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)
	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(f.Title, f.URL.String(), f.Homepage.String(), f.UpdateInterval.Seconds()); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add Feed %s to database: %s",
				f.Title,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	} else {
		var id int64

		defer rows.Close()

		if !rows.Next() {
			// CANTHAPPEN
			db.log.Printf("[ERROR] Query %s did not return a value\n",
				qid)
			return fmt.Errorf("Query %s did not return a value", qid)
		} else if err = rows.Scan(&id); err != nil {
			msg = fmt.Sprintf("Failed to get ID for newly added host %s: %s",
				f.Title,
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return errors.New(msg)
		}

		f.ID = id
		status = true
		return nil
	}
} // func (db *Database) FeedAdd(f *model.Feed) error

// FeedGetByID loads a Feed by its ID.
func (db *Database) FeedGetByID(id int64) (*model.Feed, error) {
	const qid query.ID = query.FeedGetByID
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(id); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec

	if rows.Next() {
		var (
			timestamp, interval int64
			ustr, hstr          string
			f                   = &model.Feed{ID: id}
		)

		if err = rows.Scan(&f.Title, &ustr, &hstr, &interval, &timestamp, &f.Active); err != nil {
			msg = fmt.Sprintf("Error scanning row for Feed %d: %s",
				id,
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		} else if f.URL, err = url.Parse(ustr); err != nil {
			db.log.Printf("[ERROR] Cannot parse URL %q: %s\n",
				ustr,
				err.Error())
			return nil, err
		} else if f.Homepage, err = url.Parse(hstr); err != nil {
			db.log.Printf("[ERROR] Cannot parse URL %q: %s\n",
				hstr,
				err.Error())
			return nil, err
		}

		f.LastRefresh = time.Unix(timestamp, 0)
		f.UpdateInterval = time.Second * time.Duration(interval)

		return f, nil
	}

	db.log.Printf("[INFO] Feed %d was not found in database\n", id)
	return nil, nil
} // func (db *Database) FeedGetByID(id int64) (*model.Feed, error)

// FeedGetAll loads all Feeds from the database.
func (db *Database) FeedGetAll() ([]model.Feed, error) {
	const qid query.ID = query.FeedGetAll
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var feeds = make([]model.Feed, 0, 16)

	for rows.Next() {
		var (
			timestamp, interval int64
			ustr, hstr          string
			f                   model.Feed
		)

		if err = rows.Scan(&f.ID, &f.Title, &ustr, &hstr, &interval, &timestamp, &f.Active); err != nil {
			msg = fmt.Sprintf("Error scanning row for Feed: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		} else if f.URL, err = url.Parse(ustr); err != nil {
			db.log.Printf("[ERROR] Cannot parse URL %q: %s\n",
				ustr,
				err.Error())
			return nil, err
		} else if f.Homepage, err = url.Parse(hstr); err != nil {
			db.log.Printf("[ERROR] Cannot parse URL %q: %s\n",
				hstr,
				err.Error())
			return nil, err
		}

		f.LastRefresh = time.Unix(timestamp, 0)
		f.UpdateInterval = time.Second * time.Duration(interval)
		feeds = append(feeds, f)
	}

	return feeds, nil
} // func (db *Database) FeedGetAll() ([]model.Feed, error)

// FeedGetPending load all Feeds that need to be refreshed.
func (db *Database) FeedGetPending() ([]model.Feed, error) {
	const qid query.ID = query.FeedGetPending
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var feeds = make([]model.Feed, 0, 16)

	for rows.Next() {
		var (
			timestamp, interval int64
			ustr, hstr          string
			f                   model.Feed
		)

		if err = rows.Scan(&f.ID, &f.Title, &ustr, &hstr, &interval, &timestamp, &f.Active); err != nil {
			msg = fmt.Sprintf("Error scanning row for Feed: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		} else if f.URL, err = url.Parse(ustr); err != nil {
			db.log.Printf("[ERROR] Cannot parse URL %q: %s\n",
				ustr,
				err.Error())
			return nil, err
		} else if f.Homepage, err = url.Parse(hstr); err != nil {
			db.log.Printf("[ERROR] Cannot parse URL %q: %s\n",
				hstr,
				err.Error())
			return nil, err
		}

		f.LastRefresh = time.Unix(timestamp, 0)
		f.UpdateInterval = time.Second * time.Duration(interval)
		feeds = append(feeds, f)
	}

	return feeds, nil
} // func (db *Database) FeedGetPending() ([]model.Feed, error)

// FeedUpdateRefresh updates the given Feed's LastRefresh timestamp
func (db *Database) FeedUpdateRefresh(f *model.Feed, stamp time.Time) error {
	const qid query.ID = query.FeedUpdateRefresh
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(stamp.Unix(), f.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add Feed %s to database: %s",
				f.Title,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	f.LastRefresh = stamp
	status = true
	return nil
} // func (db *Database) FeedUpdateRefresh(f *model.Feed, stamp time.Time) error

// FeedSetActive sets the given Feed's Active flag
func (db *Database) FeedSetActive(f *model.Feed, active bool) error {
	const qid query.ID = query.FeedSetActive
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(active, f.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add Feed %s to database: %s",
				f.Title,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	f.Active = active
	status = true
	return nil
} // func (db *Database) FeedSetActive(f *model.Feed, active bool) error

// FeedDelete removes the given Feed from the database.
func (db *Database) FeedDelete(f *model.Feed) error {
	const qid query.ID = query.FeedDelete
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(f.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add Feed %s to database: %s",
				f.Title,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	status = true
	return nil
} // func (db *Database) FeedDelete(f *model.Feed) error

// ItemAdd adds a news item to the database.
func (db *Database) ItemAdd(i *model.Item) error {
	const qid query.ID = query.ItemAdd
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)
	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(i.FeedID, i.URL.String(), i.Timestamp.Unix(), i.Headline, i.Description); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add Item %s to database: %s",
				i.Headline,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	} else {
		var id int64

		defer rows.Close()

		if !rows.Next() {
			// CANTHAPPEN
			db.log.Printf("[ERROR] Query %s did not return a value\n",
				qid)
			return fmt.Errorf("Query %s did not return a value", qid)
		} else if err = rows.Scan(&id); err != nil {
			msg = fmt.Sprintf("Failed to get ID for newly added host %s: %s",
				i.Headline,
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return errors.New(msg)
		}

		i.ID = id
		status = true
		return nil
	}
} // func (db *Database) ItemAdd(i *model.Item) error

// ItemDeleteByFeed removes all Items that belong to the given Feed.
func (db *Database) ItemDeleteByFeed(f *model.Feed) error {
	const qid query.ID = query.ItemDeleteByFeed
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(f.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot delete Items from Feed %s (%d): %s",
				f.Title,
				f.ID,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	status = true
	return nil
} // func (db *Database) ItemDeleteByFeed(f *model.Feed) error

// ItemExists checks if the given Item is already in the database.
func (db *Database) ItemExists(i *model.Item) (bool, error) {
	const qid query.ID = query.ItemExists
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return false, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(i.URL.String()); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return false, err
	}

	defer rows.Close() // nolint: errcheck,gosec

	if rows.Next() {
		var cnt int64

		if err = rows.Scan(&cnt); err != nil {
			msg = fmt.Sprintf("Error scanning row for Feed: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return false, errors.New(msg)
		}

		return cnt > 0, nil
	}

	db.log.Printf("[CANTHAPPEN] Query %s did not return any data looking for Item %q\n",
		qid,
		i.URL)
	return false, nil
} // func (db *Database) ItemExists(i *model.Item) (bool, error)

// ItemGetRecent loads all items newer than the given timestamp.
func (db *Database) ItemGetRecent(begin time.Time) ([]*model.Item, error) {
	const qid query.ID = query.ItemGetRecent
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(begin.Unix()); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var items = make([]*model.Item, 0, 16)

	for rows.Next() {
		var (
			timestamp int64
			ustr      string
			i         = new(model.Item)
		)

		if err = rows.Scan(&i.ID, &i.FeedID, &ustr, &timestamp, &i.Headline, &i.Description, &i.Rating); err != nil {
			msg = fmt.Sprintf("Error scanning row for Feed: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		} else if i.URL, err = url.Parse(ustr); err != nil {
			db.log.Printf("[ERROR] Cannot parse URL %q: %s\n",
				ustr,
				err.Error())
			return nil, err
		}

		i.Timestamp = time.Unix(timestamp, 0)
		items = append(items, i)
	}

	return items, nil
} // func (db *Database) ItemGetRecent(begin time.Time) ([]model.Item, error)

// ItemGetRecentPaged fetches up to cnt of the most recent news items, skipping the first offset items,
// in descending chronological order.
func (db *Database) ItemGetRecentPaged(cnt, offset int64) ([]*model.Item, error) {
	const qid query.ID = query.ItemGetRecentPaged
	var (
		err   error
		msg   string
		stmt  *sql.Stmt
		rsize int64
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(cnt, offset); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	if cnt < 0 {
		rsize = 16
	} else {
		rsize = cnt
	}

	defer rows.Close() // nolint: errcheck,gosec
	var items = make([]*model.Item, 0, rsize)

	for rows.Next() {
		var (
			timestamp int64
			ustr      string
			i         = new(model.Item)
		)

		if err = rows.Scan(&i.ID, &i.FeedID, &ustr, &timestamp, &i.Headline, &i.Description, &i.Rating); err != nil {
			msg = fmt.Sprintf("Error scanning row for Feed: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		} else if i.URL, err = url.Parse(ustr); err != nil {
			db.log.Printf("[ERROR] Cannot parse URL %q: %s\n",
				ustr,
				err.Error())
			return nil, err
		}

		i.Timestamp = time.Unix(timestamp, 0)
		items = append(items, i)
	}

	return items, nil
} // func (db *Database) ItemGetRecentPaged(cnt, offset int64) ([]model.Item, error)

// ItemGetByID loads an Item by its ID
func (db *Database) ItemGetByID(id int64) (*model.Item, error) {
	const qid query.ID = query.ItemGetByID
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(id); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec

	if rows.Next() {
		var (
			timestamp int64
			ustr      string
			i         = &model.Item{ID: id}
		)

		if err = rows.Scan(&i.FeedID, &ustr, &timestamp, &i.Headline, &i.Description, &i.Rating); err != nil {
			msg = fmt.Sprintf("Error scanning row for Feed: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		} else if i.URL, err = url.Parse(ustr); err != nil {
			db.log.Printf("[ERROR] Cannot parse URL %q: %s\n",
				ustr,
				err.Error())
			return nil, err
		}

		i.Timestamp = time.Unix(timestamp, 0)
		return i, nil
	}

	return nil, nil
} // func (db *Database) ItemGetByID(id int64) (*model.Item, error)

// ItemGetByFeed loads items from the given Feed.
func (db *Database) ItemGetByFeed(f *model.Feed, limit, offset int64) ([]*model.Item, error) {
	const qid query.ID = query.ItemGetByFeed
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(f.ID, limit, offset); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var items = make([]*model.Item, 0, 16)

	for rows.Next() {
		var (
			timestamp int64
			ustr      string
			i         = &model.Item{FeedID: f.ID}
		)

		if err = rows.Scan(&i.ID, &ustr, &timestamp, &i.Headline, &i.Description, &i.Rating); err != nil {
			msg = fmt.Sprintf("Error scanning row for Feed: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		} else if i.URL, err = url.Parse(ustr); err != nil {
			db.log.Printf("[ERROR] Cannot parse URL %q: %s\n",
				ustr,
				err.Error())
			return nil, err
		}

		i.Timestamp = time.Unix(timestamp, 0)
		items = append(items, i)
	}

	return items, nil
} // func (db *Database) ItemGetByFeed(f *model.Feed, limit, offset int64) ([]*model.Item, error)

// ItemGetByPeriod loads all Items from the given period
func (db *Database) ItemGetByPeriod(begin, end time.Time) ([]*model.Item, error) {
	const qid query.ID = query.ItemGetByPeriod
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(begin.Unix(), end.Unix()); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var items = make([]*model.Item, 0, 16)

	for rows.Next() {
		var (
			timestamp int64
			ustr      string
			i         = new(model.Item)
		)

		if err = rows.Scan(&i.ID, &i.FeedID, &ustr, &timestamp, &i.Headline, &i.Description, &i.Rating); err != nil {
			msg = fmt.Sprintf("Error scanning row for Feed: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		} else if i.URL, err = url.Parse(ustr); err != nil {
			db.log.Printf("[ERROR] Cannot parse URL %q: %s\n",
				ustr,
				err.Error())
			return nil, err
		}

		i.Timestamp = time.Unix(timestamp, 0)
		items = append(items, i)
	}

	return items, nil
} // func (db *Database) ItemGetByFeed(f *model.Feed, limit, offset int64) ([]*model.Item, error)

// ItemGetRated loads all items that have been manually rated.
func (db *Database) ItemGetRated() ([]model.Item, error) {
	const qid query.ID = query.ItemGetRated
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var items = make([]model.Item, 0, 16)

	for rows.Next() {
		var (
			timestamp int64
			ustr      string
			i         model.Item
		)

		if err = rows.Scan(&i.ID, &i.FeedID, &ustr, &timestamp, &i.Headline, &i.Description, &i.Rating); err != nil {
			msg = fmt.Sprintf("Error scanning row for Feed: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		} else if i.URL, err = url.Parse(ustr); err != nil {
			db.log.Printf("[ERROR] Cannot parse URL %q: %s\n",
				ustr,
				err.Error())
			return nil, err
		}

		i.Timestamp = time.Unix(timestamp, 0)
		items = append(items, i)
	}

	return items, nil
} // func (db *Database) ItemGetRated() ([]model.Item, error)

// ItemGetFiltered processes all(!) Items in the database, checks them against the
// given filter function, and sends the ones that pass in the given channel.
// When finished, this method closes the channel.
func (db *Database) ItemGetFiltered(q chan<- *model.Item, filter func(*model.Item) bool) error {
	const qid query.ID = query.ItemGetAll

	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	defer func() {
		close(q)
	}()

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return err
	}

	defer rows.Close() // nolint: errcheck,gosec

	for rows.Next() {
		var (
			timestamp int64
			ustr      string
			i         = new(model.Item)
		)

		if err = rows.Scan(&i.ID, &i.FeedID, &ustr, &timestamp, &i.Headline, &i.Description, &i.Rating); err != nil {
			msg = fmt.Sprintf("Error scanning row for Feed: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return errors.New(msg)
		} else if i.URL, err = url.Parse(ustr); err != nil {
			db.log.Printf("[ERROR] Cannot parse URL %q: %s\n",
				ustr,
				err.Error())
			return err
		}

		i.Timestamp = time.Unix(timestamp, 0)

		if filter(i) {
			q <- i
		}
	}

	return nil
} // func (db *Database) ItemGetFiltered(q chan<-*model.Item, filter func(*model.Item) bool) error

// ItemRate sets an Item's rating to the given value
func (db *Database) ItemRate(i *model.Item, r int8) error {
	const qid query.ID = query.ItemRate
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(r, i.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add Item %s to database: %s",
				i.Headline,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	i.Rating = r
	status = true
	return nil
} // func (db *Database) ItemRate(i *model.Item, r int64) error

// ItemUnrate resets an Item's rating to zero.
func (db *Database) ItemUnrate(i *model.Item) error {
	const qid query.ID = query.ItemUnrate
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Query(i.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add Item %s to database: %s",
				i.Headline,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	i.Rating = 0
	status = true
	return nil
} // func (db *Database) ItemUnrate(i *model.Item, r int64) error

// TagAdd adds a new Tag to the database.
func (db *Database) TagAdd(t *model.Tag) error {
	const qid query.ID = query.TagAdd
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)
	var (
		rows   *sql.Rows
		parent *int64
	)

	if t.Parent != 0 {
		parent = &t.Parent
	}

EXEC_QUERY:
	if rows, err = stmt.Query(t.Name, parent); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add Tag %s to database: %s",
				t.Name,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	} else {
		var id int64

		defer rows.Close()

		if !rows.Next() {
			// CANTHAPPEN
			db.log.Printf("[ERROR] Query %s did not return a value\n",
				qid)
			return fmt.Errorf("Query %s did not return a value", qid)
		} else if err = rows.Scan(&id); err != nil {
			msg = fmt.Sprintf("Failed to get ID for newly added Tag %s: %s",
				t.Name,
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return errors.New(msg)
		}

		t.ID = id
		status = true
		return nil
	}
} // func (db *Database) TagAdd(t *model.Tag) error

// TagGetByID loads a Tag by its ID
func (db *Database) TagGetByID(id int64) (*model.Tag, error) {
	const qid query.ID = query.TagGetByID
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(id); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec

	if rows.Next() {
		var (
			parent *int64
			t      = &model.Tag{ID: id}
		)

		if err = rows.Scan(&t.Name, &parent); err != nil {
			msg = fmt.Sprintf("Error scanning row for Feed: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		} else if parent != nil {
			t.Parent = *parent
		}

		return t, nil
	}

	return nil, nil
} // func (db *Database) TagGetByID(id int64) (*model.Tag, error)

// TagGetChildren loads all Tags that are immediate children of the given Tag.
func (db *Database) TagGetChildren(t *model.Tag) ([]*model.Tag, error) {
	const qid query.ID = query.TagGetChildren
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var tags = make([]*model.Tag, 0, 16)

	for rows.Next() {
		var (
			t = &model.Tag{Parent: t.ID}
		)

		if err = rows.Scan(&t.ID, &t.Name); err != nil {
			msg = fmt.Sprintf("Error scanning row for Feed: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		}

		tags = append(tags, t)
	}

	return tags, nil
} // func (db *Database) TagGetChildren(t *model.Tag) ([]*model.Tag, error)

// TagGetAll loads ALL Tags from the database.
func (db *Database) TagGetAll() ([]*model.Tag, error) {
	const qid query.ID = query.TagGetAll
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var tags = make([]*model.Tag, 0, 16)

	for rows.Next() {
		var (
			parent *int64
			t      = new(model.Tag)
		)

		if err = rows.Scan(&t.ID, &parent, &t.Name); err != nil {
			msg = fmt.Sprintf("Error scanning row for Feed: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		} else if parent != nil {
			t.Parent = *parent
		}

		tags = append(tags, t)
	}

	return tags, nil
} // func (db *Database) TagGetAll() ([]*model.Tag, error)

// TagGetSorted returns all Tags, sorted hierarchically.
func (db *Database) TagGetSorted() ([]*model.Tag, error) {
	const qid query.ID = query.TagGetSorted
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var tags = make([]*model.Tag, 0, 16)

	for rows.Next() {
		var (
			parent *int64
			t      = new(model.Tag)
		)

		if err = rows.Scan(&t.ID, &t.Name, &parent, &t.Level, &t.FullName); err != nil {
			msg = fmt.Sprintf("Error scanning row for Feed: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		} else if parent != nil {
			t.Parent = *parent
		}

		tags = append(tags, t)
	}

	return tags, nil
} // func (db *Database) TagGetSorted() ([]*model.Tag, error)

// TagGetItemCnt returns a map of all Tag IDs and the number of Items that have
// the Tag linked.
func (db *Database) TagGetItemCnt() (map[int64]int64, error) {
	const qid query.ID = query.TagGetItemCnt
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var tags = make(map[int64]int64, 16)

	for rows.Next() {
		var (
			id, cnt int64
		)

		if err = rows.Scan(&id, &cnt); err != nil {
			msg = fmt.Sprintf("Error scanning row for Feed: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		}

		tags[id] = cnt
	}

	return tags, nil
} // func (db *Database) TagGetItemCnt() (map[int64]int64, error)

// TagRename changes a Tag's name.
func (db *Database) TagRename(t *model.Tag, name string) error {
	const qid query.ID = query.TagRename
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(name, t.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot rename Tag %s to %s: %s",
				t.Name,
				name,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	t.Name = name
	status = true
	return nil
} // func (db *Database) TagRename(t *model.Tag, name string) error

// TagSetParent updates a Tag's parent ID.
func (db *Database) TagSetParent(t *model.Tag, parent int64) error {
	const qid query.ID = query.TagSetParent
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(parent, t.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot update parent of Tag %s (%d) to %d: %s",
				t.Name,
				t.ID,
				parent,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	t.Parent = parent
	status = true
	return nil
} // func (db *Database) TagSetParent(t *model.Tag, parent int64) error

// TagUpdate updates a Tag's name and parent at once.
func (db *Database) TagUpdate(t *model.Tag, name string, parent int64) error {
	const qid query.ID = query.TagUpdate
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
		p      *int64
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

	if parent != 0 {
		p = &parent
	}

EXEC_QUERY:
	if _, err = stmt.Exec(name, p, t.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot update parent of Tag %s (%d) to %d: %s",
				t.Name,
				t.ID,
				parent,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	t.Name = name
	t.Parent = parent
	status = true
	return nil
} // func (db *Database) TagUpdate(t *model.Tag, name string, parent int64) error

// TagDelete removes a Tag from the database.
func (db *Database) TagDelete(t *model.Tag) error {
	const qid query.ID = query.TagDelete
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
		// db.log.Printf("[INFO] Start ad-hoc transaction for adding Feed %s\n",
		// 	f.Title)
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(t.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot delete Tag %s: %s",
				t.Name,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	status = true
	return nil
} // func (db *Database) TagDelete(t *model.Tag) error

// TagLinkAdd attaches the given Tag to the given Item.
func (db *Database) TagLinkAdd(item *model.Item, tag *model.Tag) error {
	const qid query.ID = query.TagLinkAdd
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(tag.ID, item.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add Tag %s to Item %q (%d): %s",
				tag.Name,
				item.Headline,
				item.ID,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	status = true
	return nil
} // func (db *Database) TagLinkAdd(item *model.Item, tag *model.Tag) error

// TagLinkDelete removes a Tag from the given Item.
func (db *Database) TagLinkDelete(item *model.Item, tag *model.Tag) error {
	const qid query.ID = query.TagLinkDelete
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(tag.ID, item.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot remove Tag %s from Item %s (%d): %s",
				tag.Name,
				item.Headline,
				item.ID,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	status = true
	return nil
} // func (db *Database) TagLinkDelete(item *model.Item, tag *model.Tag) error

// TagLinkDeleteByFeed removes all links to Items that belong to the given
// Feed.
func (db *Database) TagLinkDeleteByFeed(f *model.Feed) error {
	const qid query.ID = query.TagLinkDeleteByFeed
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(f.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot remove Tag links to Items belonging to Feed %s (%d): %s",
				f.Title,
				f.ID,
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	status = true
	return nil
} // func (db *Database) TagLinkDeleteByFeed(f *model.Feed) error

// TagLinkGetByItem loads all Tags that are attached to the given Item.
func (db *Database) TagLinkGetByItem(item *model.Item) ([]*model.Tag, error) {
	const qid query.ID = query.TagLinkGetByItem
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(item.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var tags = make([]*model.Tag, 0, 16)

	for rows.Next() {
		var (
			parent *int64
			t      = new(model.Tag)
		)

		if err = rows.Scan(&t.ID, &parent, &t.Name); err != nil {
			msg = fmt.Sprintf("Error scanning row for Feed: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		}

		if parent != nil {
			t.Parent = *parent
		}

		tags = append(tags, t)
	}

	return tags, nil
} // func (db *Database) TagLinkGetByItem(item *model.Item) ([]*model.Tag, error)

// TagLinkGetByTag loads all Items that have the given Tag attached to them.
func (db *Database) TagLinkGetByTag(tag *model.Tag) ([]*model.Item, error) {
	const qid query.ID = query.TagLinkGetByTag
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(tag.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var items = make([]*model.Item, 0, 16)

	for rows.Next() {
		var (
			rating, stamp int64
			ustr          string
			item          = new(model.Item)
		)

		if err = rows.Scan(&item.ID, &item.FeedID, &ustr, &stamp, &item.Headline, &item.Description, &rating); err != nil {
			msg = fmt.Sprintf("Error scanning row for Item: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		}

		item.Rating = int8(rating)
		item.Timestamp = time.Unix(stamp, 0)
		if item.URL, err = url.Parse(ustr); err != nil {
			db.log.Printf("[ERROR] Invalid URL for Item %q (%d): %s\n\t%s\n",
				item.Headline,
				item.ID,
				err.Error(),
				ustr)
			return nil, err
		}

		items = append(items, item)
	}

	return items, nil
} // func (db *Database) TagLinkGetByTag(tag *model.Tag) ([]*model.Item, error)

// TagLinkGetByTagMap loads all Items that have the given Tag attached to them.
// This method returns the results as a map rather than a slice.
func (db *Database) TagLinkGetByTagMap(tag *model.Tag) (map[int64]*model.Item, error) {
	const qid query.ID = query.TagLinkGetByTag
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(tag.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var items = make(map[int64]*model.Item, 16)

	for rows.Next() {
		var (
			rating, stamp int64
			ustr          string
			item          = new(model.Item)
		)

		if err = rows.Scan(&item.ID, &item.FeedID, &ustr, &stamp, &item.Headline, &item.Description, &rating); err != nil {
			msg = fmt.Sprintf("Error scanning row for Item: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		}

		item.Rating = int8(rating)
		item.Timestamp = time.Unix(stamp, 0)
		if item.URL, err = url.Parse(ustr); err != nil {
			db.log.Printf("[ERROR] Invalid URL for Item %q (%d): %s\n\t%s\n",
				item.Headline,
				item.ID,
				err.Error(),
				ustr)
			return nil, err
		}

		// items = append(items, item)
		items[item.ID] = item
	}

	return items, nil
} // func (db *Database) TagLinkGetByTag(tag *model.Tag) ([]*model.Item, error)

// SearchAdd enters a Search query into the database.
func (db *Database) SearchAdd(s *model.Search) error {
	const qid query.ID = query.SearchAdd
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)
	var (
		rows   *sql.Rows
		tagIDs []string
		tags   string
	)

	tagIDs = make([]string, len(s.Tags))

	for idx, tid := range s.Tags {
		tagIDs[idx] = strconv.FormatInt(tid, 10)
	}

	tags = strings.Join(tagIDs, ",")

EXEC_QUERY:
	if rows, err = stmt.Query(s.Title, s.TimeCreated.Unix(), tags, s.TagsAll, s.QueryString, s.Regex); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add Search to Database: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	defer rows.Close()

	if !rows.Next() {
		err = fmt.Errorf("Query %s did not return a result set", qid)
		db.log.Printf("[ERROR] %s\n", err.Error())
		return err
	} else if err = rows.Scan(&s.ID); err != nil {
		db.log.Printf("[ERROR] Failed to scan Search ID from result set: %s\n",
			err.Error())
		return err
	}

	status = true
	return nil
} // func (db *Database) SearchAdd(s *model.Search) error

// SearchDelete removes a search query from the database.
func (db *Database) SearchDelete(s *model.Search) error {
	const qid query.ID = query.SearchDelete
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)

EXEC_QUERY:
	if _, err = stmt.Exec(s.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add Search to Database: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	status = true
	return nil
} // func (db *Database) SearchDelete(s *model.Search) error

// SearchGetByID looks up a search query by its ID. If the search has been
// finished, the results are included.
func (db *Database) SearchGetByID(id int64) (*model.Search, error) {
	const qid query.ID = query.SearchGetByID
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(id); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec

	if rows.Next() {
		var (
			s                   = &model.Search{ID: id}
			tcreated            int64
			tstarted, tfinished *int64
			tagStr, resultStr   string
			tags, results       []string
		)

		if err = rows.Scan(&s.Title, &tcreated, &tstarted, &tfinished, &s.Status, &s.Message, &tagStr, &s.TagsAll, &s.QueryString, &s.Regex, &resultStr); err != nil {
			msg = fmt.Sprintf("Error scanning row for Search %d: %s",
				id,
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		}

		s.TimeCreated = time.Unix(tcreated, 0)
		if tstarted != nil {
			s.TimeStarted = time.Unix(*tstarted, 0)
		}
		if tfinished != nil {
			s.TimeFinished = time.Unix(*tfinished, 0)
		}

		tags = strings.Split(tagStr, ",")
		results = strings.Split(resultStr, ",")

		if len(tags) > 0 {
			s.Tags = make([]int64, len(tags))

			for idx, t := range tags {
				var tid int64
				if tid, err = strconv.ParseInt(t, 10, 64); err != nil {
					db.log.Printf("[ERROR] Cannot parse Tag ID %q: %s\n",
						t,
						err.Error())
					return nil, err
				}
				s.Tags[idx] = tid
			}
		}

		if len(results) > 0 {
			s.Results = make([]*model.Item, len(results))

			for idx, r := range results {
				var rid int64
				if rid, err = strconv.ParseInt(r, 10, 64); err != nil {
					db.log.Printf("[ERROR] Cannot parse Item ID %q: %s\n",
						r,
						err.Error())
					return nil, err
				} else if s.Results[idx], err = db.ItemGetByID(rid); err != nil {
					db.log.Printf("[ERROR] Failed to load Item %d: %s\n",
						rid,
						err.Error())
					return nil, err
				}
			}
		}

		return s, nil
	}

	return nil, nil
} // func (db *Database) SearchGetByID(id int64) (*model.Search, error)

// SearchGetNextPending returns the oldest Search Query in the database that
// has not been started, yet.
func (db *Database) SearchGetNextPending() (*model.Search, error) {
	const qid query.ID = query.SearchGetNextPending
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec

	if !rows.Next() {
		// db.log.Println("[DEBUG] No pending Search was found in database.")
		return nil, nil
	}

	var (
		s                                = new(model.Search)
		tcreated, periodBegin, periodEnd int64
		tagStr                           string
		tags                             []string
	)

	if err = rows.Scan(&s.ID, &s.Title, &tcreated, &tagStr, &s.TagsAll, &s.FilterByPeriod, &periodBegin, &periodEnd, &s.QueryString, &s.Regex); err != nil {
		msg = fmt.Sprintf("Error scanning row for Search: %s",
			err.Error())
		db.log.Printf("[ERROR] %s\n", msg)
		return nil, errors.New(msg)
	}

	s.TimeCreated = time.Unix(tcreated, 0)
	s.FilterPeriod[0] = time.Unix(periodBegin, 0)
	s.FilterPeriod[1] = time.Unix(periodEnd, 0)

	tags = strings.Split(tagStr, ",")

	if len(tags) > 0 {
		s.Tags = make([]int64, len(tags))

		for idx, t := range tags {
			var tid int64
			if tid, err = strconv.ParseInt(t, 10, 64); err != nil {
				db.log.Printf("[ERROR] Cannot parse Tag ID %q: %s\n",
					t,
					err.Error())
				return nil, err
			}
			s.Tags[idx] = tid
		}
	}

	return s, nil
} // func (db *Database) SearchGetNextPending() (*model.Search, error)

// SearchGetActive returns all Search queries that have been marked as started but not finished.
func (db *Database) SearchGetActive() ([]*model.Search, error) {
	const qid query.ID = query.SearchGetActive
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var queries = make([]*model.Search, 0, 8)

	for rows.Next() {
		var (
			s                                = new(model.Search)
			tcreated, periodBegin, periodEnd int64
			tstarted                         *int64
			tagStr, resultStr                string
			tags, results                    []string
		)

		if err = rows.Scan(&s.Title, &tcreated, &tstarted, &s.Status, &s.Message, &tagStr, &s.TagsAll, &s.FilterByPeriod, &periodBegin, &periodEnd, &s.QueryString, &s.Regex, &resultStr); err != nil {
			msg = fmt.Sprintf("Error scanning row for pending Search queries: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		}

		s.TimeCreated = time.Unix(tcreated, 0)
		s.FilterPeriod[0] = time.Unix(periodBegin, 0)
		s.FilterPeriod[1] = time.Unix(periodEnd, 0)
		if tstarted != nil {
			s.TimeStarted = time.Unix(*tstarted, 0)
		}

		tags = strings.Split(tagStr, ",")
		results = strings.Split(resultStr, ",")

		if len(tags) > 0 {
			s.Tags = make([]int64, len(tags))

			for idx, t := range tags {
				var tid int64
				if tid, err = strconv.ParseInt(t, 10, 64); err != nil {
					db.log.Printf("[ERROR] Cannot parse Tag ID %q: %s\n",
						t,
						err.Error())
					return nil, err
				}
				s.Tags[idx] = tid
			}
		}

		if len(results) > 0 {
			s.Results = make([]*model.Item, len(results))

			for idx, r := range results {
				var rid int64
				if rid, err = strconv.ParseInt(r, 10, 64); err != nil {
					db.log.Printf("[ERROR] Cannot parse Item ID %q: %s\n",
						r,
						err.Error())
					return nil, err
				} else if s.Results[idx], err = db.ItemGetByID(rid); err != nil {
					db.log.Printf("[ERROR] Failed to load Item %d: %s\n",
						rid,
						err.Error())
					return nil, err
				}
			}
		}

		queries = append(queries, s)
	}

	return queries, nil
} // func (db *Database) SearchGetActive() ([]*model.Search, error)

// SearchGetAll loads all existing search queries.
func (db *Database) SearchGetAll() ([]*model.Search, error) {
	const qid query.ID = query.SearchGetAll
	var (
		err  error
		msg  string
		stmt *sql.Stmt
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return nil, err
	} else if db.tx != nil {
		stmt = db.tx.Stmt(stmt)
	}

	var rows *sql.Rows

EXEC_QUERY:
	if rows, err = stmt.Query(); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		}

		return nil, err
	}

	defer rows.Close() // nolint: errcheck,gosec
	var queries = make([]*model.Search, 0, 8)

	for rows.Next() {
		var (
			s                      = new(model.Search)
			tcreated               int64
			tstarted, tfinished    *int64
			tagStr, resultStr      string
			periodBegin, periodEnd int64
			tags, results          []string
		)

		if err = rows.Scan(&s.ID, &s.Title, &tcreated, &tstarted, &tfinished, &s.Status, &s.Message, &tagStr, &s.TagsAll, &s.FilterByPeriod, &periodBegin, &periodEnd, &s.QueryString, &s.Regex, &resultStr); err != nil {
			msg = fmt.Sprintf("Error scanning row for pending Search queries: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", msg)
			return nil, errors.New(msg)
		}

		s.TimeCreated = time.Unix(tcreated, 0)
		s.FilterPeriod[0] = time.Unix(periodBegin, 0)
		s.FilterPeriod[1] = time.Unix(periodEnd, 0)
		if tstarted != nil {
			s.TimeStarted = time.Unix(*tstarted, 0)
		}
		if tfinished != nil {
			s.TimeFinished = time.Unix(*tfinished, 0)
		}

		tags = strings.Split(tagStr, ",")
		results = strings.Split(resultStr, ",")

		if len(tags) > 0 {
			s.Tags = make([]int64, len(tags))

			for idx, t := range tags {
				var tid int64
				if tid, err = strconv.ParseInt(t, 10, 64); err != nil {
					db.log.Printf("[ERROR] Cannot parse Tag ID %q: %s\n",
						t,
						err.Error())
					return nil, err
				}
				s.Tags[idx] = tid
			}
		}

		if len(results) > 0 {
			s.Results = make([]*model.Item, len(results))

			for idx, r := range results {
				var rid int64
				if rid, err = strconv.ParseInt(r, 10, 64); err != nil {
					db.log.Printf("[ERROR] Cannot parse Item ID %q: %s\n",
						r,
						err.Error())
					return nil, err
				} else if s.Results[idx], err = db.ItemGetByID(rid); err != nil {
					db.log.Printf("[ERROR] Failed to load Item %d: %s\n",
						rid,
						err.Error())
					return nil, err
				}
			}
		}

		queries = append(queries, s)
	}

	return queries, nil
} // func (db *Database) SearchGetAll() ([]*model.Search, error)

// SearchStart sets the start time of the given Search query to the current
// time, marking it as active.
func (db *Database) SearchStart(s *model.Search) error {
	const qid query.ID = query.SearchStart
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)
	var startStamp = time.Now()

EXEC_QUERY:
	if _, err = stmt.Exec(startStamp.Unix(), s.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add Search to Database: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	s.TimeStarted = startStamp
	status = true
	return nil
} // func (db *Database) SearchStart(s *model.Search) error

// SearchFinish sets the Finished timestamp of the given Search query to the
// current time, marking it as finished. It also updates the status, message, and
// results fields accordingly.
func (db *Database) SearchFinish(s *model.Search) error {
	const qid query.ID = query.SearchFinish
	var (
		err    error
		msg    string
		stmt   *sql.Stmt
		tx     *sql.Tx
		status bool
	)

	if stmt, err = db.getQuery(qid); err != nil {
		db.log.Printf("[ERROR] Cannot prepare query %s: %s\n",
			qid,
			err.Error())
		return err
	} else if db.tx != nil {
		tx = db.tx
	} else {
	BEGIN_AD_HOC:
		if tx, err = db.db.Begin(); err != nil {
			if worthARetry(err) {
				waitForRetry()
				goto BEGIN_AD_HOC
			} else {
				msg = fmt.Sprintf("Error starting transaction: %s\n",
					err.Error())
				db.log.Printf("[ERROR] %s\n", msg)
				return errors.New(msg)
			}

		} else {
			defer func() {
				var err2 error
				if status {
					if err2 = tx.Commit(); err2 != nil {
						db.log.Printf("[ERROR] Failed to commit ad-hoc transaction: %s\n",
							err2.Error())
					}
				} else if err2 = tx.Rollback(); err2 != nil {
					db.log.Printf("[ERROR] Rollback of ad-hoc transaction failed: %s\n",
						err2.Error())
				}
			}()
		}
	}

	stmt = tx.Stmt(stmt)
	var (
		finishStamp = time.Now()
		resultsList = make([]string, len(s.Results))
		resultStr   string
	)

	for idx, item := range s.Results {
		resultsList[idx] = strconv.FormatInt(item.ID, 10)
	}

	resultStr = strings.Join(resultsList, ",")

EXEC_QUERY:
	if _, err = stmt.Exec(finishStamp.Unix(), s.Status, s.Message, resultStr, s.ID); err != nil {
		if worthARetry(err) {
			waitForRetry()
			goto EXEC_QUERY
		} else {
			err = fmt.Errorf("Cannot add Search to Database: %s",
				err.Error())
			db.log.Printf("[ERROR] %s\n", err.Error())
			return err
		}
	}

	s.TimeFinished = finishStamp
	status = true
	return nil
} // func (db *Database) SearchFinish(s *model.Search) error
