// Copyright 2017 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tests

import (
	"database/sql"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/imkos/xorm"
	"github.com/imkos/xorm/caches"
	"github.com/imkos/xorm/dialects"
	"github.com/imkos/xorm/log"
	"github.com/imkos/xorm/names"
	"github.com/imkos/xorm/schemas"
)

var (
	testEngine xorm.EngineInterface
	dbType     string
	connString string

	db                 = flag.String("db", "sqlite3", "the tested database")
	showSQL            = flag.Bool("show_sql", true, "show generated SQLs")
	ptrConnStr         = flag.String("conn_str", "./test.db?cache=shared&mode=rwc", "test database connection string")
	mapType            = flag.String("map_type", "snake", "indicate the name mapping")
	cacheFlag          = flag.Bool("cache", false, "if enable cache")
	cluster            = flag.Bool("cluster", false, "if this is a cluster")
	splitter           = flag.String("splitter", ";", "the splitter on connstr for cluster")
	schema             = flag.String("schema", "", "specify the schema")
	ignoreSelectUpdate = flag.Bool("ignore_select_update", false, "ignore select update if implementation difference, only for tidb")
	ingoreUpdateLimit  = flag.Bool("ignore_update_limit", false, "ignore update limit if implementation difference, only for cockroach")
	doNVarcharTest     = flag.Bool("do_nvarchar_override_test", false, "do nvarchar override test in sync table, only for mssql")
	quotePolicyStr     = flag.String("quote", "always", "quote could be always, none, reversed")
	defaultVarchar     = flag.String("default_varchar", "varchar", "default varchar type, mssql only, could be varchar or nvarchar, default is varchar")
	defaultChar        = flag.String("default_char", "char", "default char type, mssql only, could be char or nchar, default is char")
	collation          = flag.String("collation", "", "default database collation, it's different for different database")
	tableMapper        names.Mapper
	colMapper          names.Mapper
)

func createEngine(dbType, connStr string) error {
	if testEngine == nil {
		var err error

		if !*cluster {
			switch schemas.DBType(strings.ToLower(dbType)) {
			case schemas.MSSQL:
				db, err := sql.Open(dbType, strings.ReplaceAll(connStr, "xorm_test", "master"))
				if err != nil {
					return err
				}
				createDBSQL := "If(db_id(N'xorm_test') IS NULL) BEGIN CREATE DATABASE xorm_test"
				if collation != nil && *collation != "" {
					createDBSQL += fmt.Sprintf(" COLLATE %s", *collation)
				}
				createDBSQL += "; END;"
				if _, err = db.Exec(createDBSQL); err != nil {
					return fmt.Errorf("db.Exec: %v", err)
				}
				db.Close()
				*ignoreSelectUpdate = true
			case schemas.POSTGRES:
				db, err := sql.Open(dbType, strings.ReplaceAll(connStr, "xorm_test", "postgres"))
				if err != nil {
					return err
				}
				rows, err := db.Query("SELECT 1 FROM pg_database WHERE datname = 'xorm_test'")
				if err != nil {
					return fmt.Errorf("db.Query: %v", err)
				}
				defer rows.Close()

				if !rows.Next() {
					if _, err = db.Exec("CREATE DATABASE xorm_test"); err != nil {
						return fmt.Errorf("CREATE DATABASE: %v", err)
					}
				}
				if *schema != "" {
					db.Close()
					db, err = sql.Open(dbType, connStr)
					if err != nil {
						return err
					}
					defer db.Close()
					if _, err = db.Exec("CREATE SCHEMA IF NOT EXISTS " + *schema); err != nil {
						return fmt.Errorf("CREATE SCHEMA: %v", err)
					}
				}
				db.Close()
				*ignoreSelectUpdate = true
			case schemas.MYSQL:
				db, err := sql.Open(dbType, strings.ReplaceAll(connStr, "xorm_test", "mysql"))
				if err != nil {
					return err
				}
				if _, err = db.Exec("CREATE DATABASE IF NOT EXISTS xorm_test"); err != nil {
					return fmt.Errorf("db.Exec: %v", err)
				}
				db.Close()
			case schemas.SQLITE, "sqlite":
				u, err := url.Parse(connStr)
				if err != nil {
					return err
				}
				connStr = u.Path
				*ignoreSelectUpdate = true
			default:
				*ignoreSelectUpdate = true
			}

			testEngine, err = xorm.NewEngine(dbType, connStr)
		} else {
			testEngine, err = xorm.NewEngineGroup(dbType, strings.Split(connStr, *splitter))
			if dbType != "mysql" && dbType != "mymysql" {
				*ignoreSelectUpdate = true
			}
		}
		if err != nil {
			return err
		}

		if *schema != "" {
			testEngine.SetSchema(*schema)
		}
		testEngine.ShowSQL(*showSQL)
		testEngine.SetLogLevel(log.LOG_DEBUG)
		if *cacheFlag {
			cacher := caches.NewLRUCacher(caches.NewMemoryStore(), 100000)
			testEngine.SetDefaultCacher(cacher)
		}

		if len(*mapType) > 0 {
			switch *mapType {
			case "snake":
				testEngine.SetMapper(names.SnakeMapper{})
			case "same":
				testEngine.SetMapper(names.SameMapper{})
			case "gonic":
				testEngine.SetMapper(names.LintGonicMapper)
			}
		}

		if *quotePolicyStr == "none" {
			testEngine.SetQuotePolicy(dialects.QuotePolicyNone)
		} else if *quotePolicyStr == "reserved" {
			testEngine.SetQuotePolicy(dialects.QuotePolicyReserved)
		} else {
			testEngine.SetQuotePolicy(dialects.QuotePolicyAlways)
		}

		testEngine.Dialect().SetParams(map[string]string{
			"DEFAULT_VARCHAR": *defaultVarchar,
			"DEFAULT_CHAR":    *defaultChar,
		})
	}

	tableMapper = testEngine.GetTableMapper()
	colMapper = testEngine.GetColumnMapper()

	tables, err := testEngine.DBMetas()
	if err != nil {
		return err
	}
	tableNames := make([]interface{}, 0, len(tables))
	for _, table := range tables {
		tableNames = append(tableNames, table.Name)
	}
	return testEngine.DropTables(tableNames...)
}

// PrepareEngine prepare tests ORM engine
func PrepareEngine() error {
	return createEngine(dbType, connString)
}

// MainTest the tests entrance
func MainTest(m *testing.M) {
	flag.Parse()

	dbType = *db
	if *db == "sqlite3" {
		if ptrConnStr == nil {
			connString = "./test_sqlite3.db?cache=shared&mode=rwc"
		} else {
			connString = *ptrConnStr
		}
	} else if *db == "sqlite" {
		if ptrConnStr == nil {
			connString = "./test_sqlite.db?cache=shared&mode=rwc"
		} else {
			connString = *ptrConnStr
		}
	} else {
		if ptrConnStr == nil {
			fmt.Println("you should indicate conn string")
			return
		}
		connString = *ptrConnStr
	}

	dbs := strings.Split(*db, "::")
	conns := strings.Split(connString, "::")

	var res int
	for i := 0; i < len(dbs); i++ {
		dbType = dbs[i]
		connString = conns[i]
		testEngine = nil
		fmt.Println("testing", dbType, connString)

		if err := PrepareEngine(); err != nil {
			fmt.Println(err)
			os.Exit(1)
			return
		}

		code := m.Run()
		if code > 0 {
			res = code
		}
	}

	os.Exit(res)
}
