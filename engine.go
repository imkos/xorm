// Copyright 2015 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/imkos/xorm/caches"
	"github.com/imkos/xorm/contexts"
	"github.com/imkos/xorm/core"
	"github.com/imkos/xorm/dialects"
	"github.com/imkos/xorm/internal/utils"
	"github.com/imkos/xorm/log"
	"github.com/imkos/xorm/names"
	"github.com/imkos/xorm/schemas"
	"github.com/imkos/xorm/tags"
)

// Engine is the major struct of xorm, it means a database manager.
// Commonly, an application only need one engine
type Engine struct {
	cacherMgr      *caches.Manager
	defaultContext context.Context
	dialect        dialects.Dialect
	driver         dialects.Driver
	engineGroup    *EngineGroup
	logger         log.ContextLogger
	tagParser      *tags.Parser
	db             *core.DB

	driverName     string
	dataSourceName string

	TZLocation *time.Location // The timezone of the application
	DatabaseTZ *time.Location // The timezone of the database

	logSessionID bool // create session id
}

// NewEngine new a db manager according to the parameter. Currently support four
// drivers
func NewEngine(driverName string, dataSourceName string, driverOptions ...func(db *sql.DB) error) (*Engine, error) {
	dialect, err := dialects.OpenDialect(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}

	db, err := core.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}

	for _, driverOption := range driverOptions {
		if err := driverOption(db.DB); err != nil {
			return nil, err
		}
	}

	return newEngine(driverName, dataSourceName, dialect, db)
}

func newEngine(driverName, dataSourceName string, dialect dialects.Dialect, db *core.DB) (*Engine, error) {
	cacherMgr := caches.NewManager()
	mapper := names.NewCacheMapper(new(names.SnakeMapper))
	tagParser := tags.NewParser("xorm", dialect, mapper, mapper, cacherMgr)

	engine := &Engine{
		dialect:        dialect,
		driver:         dialects.QueryDriver(driverName),
		TZLocation:     time.Local,
		defaultContext: context.Background(),
		cacherMgr:      cacherMgr,
		tagParser:      tagParser,
		driverName:     driverName,
		dataSourceName: dataSourceName,
		db:             db,
		logSessionID:   false,
	}

	if dialect.URI().DBType == schemas.SQLITE {
		engine.DatabaseTZ = time.UTC
	} else {
		engine.DatabaseTZ = time.Local
	}

	logger := log.NewSimpleLogger(os.Stdout)
	logger.SetLevel(log.LOG_INFO)
	engine.SetLogger(log.NewLoggerAdapter(logger))

	runtime.SetFinalizer(engine, func(engine *Engine) {
		_ = engine.Close()
	})

	return engine, nil
}

// NewEngineWithParams new a db manager with params. The params will be passed to dialects.
func NewEngineWithParams(driverName string, dataSourceName string, params map[string]string) (*Engine, error) {
	engine, err := NewEngine(driverName, dataSourceName)
	engine.dialect.SetParams(params)
	return engine, err
}

// NewEngineWithDB new a db manager with db. The params will be passed to db.
func NewEngineWithDB(driverName string, dataSourceName string, db *core.DB) (*Engine, error) {
	dialect, err := dialects.OpenDialect(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	return newEngine(driverName, dataSourceName, dialect, db)
}

// NewEngineWithDialectAndDB new a db manager according to the parameter.
// If you do not want to use your own dialect or db, please use NewEngine.
// For creating dialect, you can call dialects.OpenDialect. And, for creating db,
// you can call core.Open or core.FromDB.
func NewEngineWithDialectAndDB(driverName, dataSourceName string, dialect dialects.Dialect, db *core.DB) (*Engine, error) {
	return newEngine(driverName, dataSourceName, dialect, db)
}

// EnableSessionID if enable session id
func (engine *Engine) EnableSessionID(enable bool) {
	engine.logSessionID = enable
}

// SetCacher sets cacher for the table
func (engine *Engine) SetCacher(tableName string, cacher caches.Cacher) {
	engine.cacherMgr.SetCacher(tableName, cacher)
}

// GetCacher returns the cachher of the special table
func (engine *Engine) GetCacher(tableName string) caches.Cacher {
	return engine.cacherMgr.GetCacher(tableName)
}

// SetQuotePolicy sets the special quote policy
func (engine *Engine) SetQuotePolicy(quotePolicy dialects.QuotePolicy) {
	engine.dialect.SetQuotePolicy(quotePolicy)
}

// BufferSize sets buffer size for iterate
func (engine *Engine) BufferSize(size int) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.BufferSize(size)
}

// ShowSQL show SQL statement or not on logger if log level is great than INFO
func (engine *Engine) ShowSQL(show ...bool) {
	engine.logger.ShowSQL(show...)
	engine.DB().Logger = engine.logger
}

// Logger return the logger interface
func (engine *Engine) Logger() log.ContextLogger {
	return engine.logger
}

// SetLogger set the new logger
func (engine *Engine) SetLogger(logger interface{}) {
	var realLogger log.ContextLogger
	switch t := logger.(type) {
	case log.ContextLogger:
		realLogger = t
	case log.Logger:
		realLogger = log.NewLoggerAdapter(t)
	default:
		panic("logger should implement either log.ContextLogger or log.Logger")
	}
	engine.logger = realLogger
	engine.DB().Logger = realLogger
}

// SetLogLevel sets the logger level
func (engine *Engine) SetLogLevel(level log.LogLevel) {
	engine.logger.SetLevel(level)
}

// SetDisableGlobalCache disable global cache or not
func (engine *Engine) SetDisableGlobalCache(disable bool) {
	engine.cacherMgr.SetDisableGlobalCache(disable)
}

// DriverName return the current sql driver's name
func (engine *Engine) DriverName() string {
	return engine.driverName
}

// DataSourceName return the current connection string
func (engine *Engine) DataSourceName() string {
	return engine.dataSourceName
}

// SetMapper set the name mapping rules
func (engine *Engine) SetMapper(mapper names.Mapper) {
	engine.SetTableMapper(mapper)
	engine.SetColumnMapper(mapper)
}

// SetTableMapper set the table name mapping rule
func (engine *Engine) SetTableMapper(mapper names.Mapper) {
	engine.tagParser.SetTableMapper(mapper)
}

// SetColumnMapper set the column name mapping rule
func (engine *Engine) SetColumnMapper(mapper names.Mapper) {
	engine.tagParser.SetColumnMapper(mapper)
}

// SetTagIdentifier set the tag identifier
func (engine *Engine) SetTagIdentifier(tagIdentifier string) {
	engine.tagParser.SetIdentifier(tagIdentifier)
}

// Quote Use QuoteStr quote the string sql
func (engine *Engine) Quote(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == 0 {
		return value
	}

	buf := strings.Builder{}
	engine.QuoteTo(&buf, value)

	return buf.String()
}

// QuoteTo quotes string and writes into the buffer
func (engine *Engine) QuoteTo(buf *strings.Builder, value string) {
	if buf == nil {
		return
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	engine.dialect.Quoter().QuoteTo(buf, value)
}

// SQLType A simple wrapper to dialect's core.SqlType method
func (engine *Engine) SQLType(c *schemas.Column) string {
	return engine.dialect.SQLType(c)
}

// SetConnMaxLifetime sets the maximum amount of time a connection may be reused.
func (engine *Engine) SetConnMaxLifetime(d time.Duration) {
	engine.DB().SetConnMaxLifetime(d)
}

// SetConnMaxIdleTime sets the maximum amount of time a connection may be idle.
func (engine *Engine) SetConnMaxIdleTime(d time.Duration) {
	engine.DB().SetConnMaxIdleTime(d)
}

// SetMaxOpenConns is only available for go 1.2+
func (engine *Engine) SetMaxOpenConns(conns int) {
	engine.DB().SetMaxOpenConns(conns)
}

// SetMaxIdleConns set the max idle connections on pool, default is 2
func (engine *Engine) SetMaxIdleConns(conns int) {
	engine.DB().SetMaxIdleConns(conns)
}

// SetDefaultCacher set the default cacher. Xorm's default not enable cacher.
func (engine *Engine) SetDefaultCacher(cacher caches.Cacher) {
	engine.cacherMgr.SetDefaultCacher(cacher)
}

// GetDefaultCacher returns the default cacher
func (engine *Engine) GetDefaultCacher() caches.Cacher {
	return engine.cacherMgr.GetDefaultCacher()
}

// NoCache If you has set default cacher, and you want temporilly stop use cache,
// you can use NoCache()
func (engine *Engine) NoCache() *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.NoCache()
}

// NoCascade If you do not want to auto cascade load object
func (engine *Engine) NoCascade() *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.NoCascade()
}

// MapCacher Set a table use a special cacher
func (engine *Engine) MapCacher(bean interface{}, cacher caches.Cacher) error {
	engine.SetCacher(dialects.FullTableName(engine.dialect, engine.GetTableMapper(), bean, true), cacher)
	return nil
}

// NewDB provides an interface to operate database directly
func (engine *Engine) NewDB() (*core.DB, error) {
	return core.Open(engine.driverName, engine.dataSourceName)
}

// DB return the wrapper of sql.DB
func (engine *Engine) DB() *core.DB {
	return engine.db
}

// Dialect return database dialect
func (engine *Engine) Dialect() dialects.Dialect {
	return engine.dialect
}

// NewSession New a session
func (engine *Engine) NewSession() *Session {
	return newSession(engine)
}

// Close the engine
func (engine *Engine) Close() error {
	return engine.DB().Close()
}

// Ping tests if database is alive
func (engine *Engine) Ping() error {
	session := engine.NewSession()
	defer session.Close()
	return session.Ping()
}

// SQL method let's you manually write raw SQL and operate
// For example:
//
//	engine.SQL("select * from user").Find(&users)
//
// This    code will execute "select * from user" and set the records to users
func (engine *Engine) SQL(query interface{}, args ...interface{}) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.SQL(query, args...)
}

// NoAutoTime Default if your struct has "created" or "updated" filed tag, the fields
// will automatically be filled with current time when Insert or Update
// invoked. Call NoAutoTime if you dont' want to fill automatically.
func (engine *Engine) NoAutoTime() *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.NoAutoTime()
}

// NoAutoCondition disable auto generate Where condition from bean or not
func (engine *Engine) NoAutoCondition(no ...bool) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.NoAutoCondition(no...)
}

func (engine *Engine) loadTableInfo(ctx context.Context, table *schemas.Table) error {
	colSeq, cols, err := engine.dialect.GetColumns(engine.db, ctx, table.Name)
	if err != nil {
		return err
	}
	for _, name := range colSeq {
		table.AddColumn(cols[name])
	}
	indexes, err := engine.dialect.GetIndexes(engine.db, ctx, table.Name)
	if err != nil {
		return err
	}
	table.Indexes = indexes

	var seq int
	for _, index := range indexes {
		for _, name := range index.Cols {
			parts := strings.Split(strings.TrimSpace(name), " ")
			if len(parts) > 1 {
				if parts[1] == "DESC" {
					seq = 1
				} else if parts[1] == "ASC" {
					seq = 0
				}
			}
			colName := strings.Trim(parts[0], `"`)
			if col := table.GetColumn(colName); col != nil {
				col.Indexes[index.Name] = index.Type
			} else {
				return fmt.Errorf("Unknown col %s seq %d, in index %v of table %v, columns %v", name, seq, index.Name, table.Name, table.ColumnsSeq())
			}
		}
	}
	return nil
}

// DBMetas Retrieve all tables, columns, indexes' informations from database.
func (engine *Engine) DBMetas() ([]*schemas.Table, error) {
	tables, err := engine.dialect.GetTables(engine.db, engine.defaultContext)
	if err != nil {
		return nil, err
	}

	for _, table := range tables {
		if err = engine.loadTableInfo(engine.defaultContext, table); err != nil {
			return nil, err
		}
	}
	return tables, nil
}

// DumpAllToFile dump database all table structs and data to a file
func (engine *Engine) DumpAllToFile(fp string, tp ...schemas.DBType) error {
	f, err := os.Create(fp)
	if err != nil {
		return err
	}
	defer f.Close()
	return engine.DumpAll(f, tp...)
}

// DumpAll dump database all table structs and data to w
func (engine *Engine) DumpAll(w io.Writer, tp ...schemas.DBType) error {
	tables, err := engine.DBMetas()
	if err != nil {
		return err
	}
	return engine.DumpTables(tables, w, tp...)
}

// DumpTablesToFile dump specified tables to SQL file.
func (engine *Engine) DumpTablesToFile(tables []*schemas.Table, fp string, tp ...schemas.DBType) error {
	f, err := os.Create(fp)
	if err != nil {
		return err
	}
	defer f.Close()
	return engine.DumpTables(tables, f, tp...)
}

// DumpTables dump specify tables to io.Writer
func (engine *Engine) DumpTables(tables []*schemas.Table, w io.Writer, tp ...schemas.DBType) error {
	return engine.dumpTables(context.Background(), tables, w, tp...)
}

func formatBool(s bool, dstDialect dialects.Dialect) string {
	if dstDialect.URI().DBType != schemas.POSTGRES {
		if s {
			return "1"
		}
		return "0"
	}
	return strconv.FormatBool(s)
}

var controlCharactersRe = regexp.MustCompile(`[\x00-\x1f\x7f]+`)

// dumpTables dump database all table structs and data to w with specify db type
func (engine *Engine) dumpTables(ctx context.Context, tables []*schemas.Table, w io.Writer, tp ...schemas.DBType) error {
	var dstDialect dialects.Dialect
	if len(tp) == 0 {
		dstDialect = engine.dialect
	} else {
		dstDialect = dialects.QueryDialect(tp[0])
		if dstDialect == nil {
			return fmt.Errorf("unsupported database type %v", tp[0])
		}

		uri := engine.dialect.URI()
		destURI := dialects.URI{
			DBType: tp[0],
			DBName: uri.DBName,
			// DO NOT SET SCHEMA HERE
		}
		if tp[0] == schemas.POSTGRES {
			destURI.Schema = engine.dialect.URI().Schema
		}
		if err := dstDialect.Init(&destURI); err != nil {
			return err
		}
	}
	cacherMgr := caches.NewManager()
	dstTableCache := tags.NewParser("xorm", dstDialect, engine.GetTableMapper(), engine.GetColumnMapper(), cacherMgr)

	_, err := io.WriteString(w, fmt.Sprintf("/*Generated by xorm %s, from %s to %s*/\n\n",
		time.Now().In(engine.TZLocation).Format("2006-01-02 15:04:05"), engine.dialect.URI().DBType, dstDialect.URI().DBType))
	if err != nil {
		return err
	}

	if dstDialect.URI().DBType == schemas.MYSQL {
		// For MySQL set NO_BACKLASH_ESCAPES so that strings work properly
		if _, err := io.WriteString(w, "SET sql_mode='NO_BACKSLASH_ESCAPES';\n"); err != nil {
			return err
		}
	}

	for i, table := range tables {
		dstTable := table
		if table.Type != nil {
			dstTable, err = dstTableCache.Parse(reflect.New(table.Type).Elem())
			if err != nil {
				engine.logger.Errorf("Unable to infer table for %s in new dialect. Error: %v", table.Name)
				dstTable = table
			}
		}

		dstTableName := dstTable.Name
		quoter := dstDialect.Quoter().Quote
		quotedDstTableName := quoter(dstTable.Name)
		if dstDialect.URI().Schema != "" {
			dstTableName = fmt.Sprintf("%s.%s", dstDialect.URI().Schema, dstTable.Name)
			quotedDstTableName = fmt.Sprintf("%s.%s", quoter(dstDialect.URI().Schema), quoter(dstTable.Name))
		}
		originalTableName := table.Name
		if engine.dialect.URI().Schema != "" {
			originalTableName = fmt.Sprintf("%s.%s", engine.dialect.URI().Schema, table.Name)
		}
		if i > 0 {
			_, err = io.WriteString(w, "\n")
			if err != nil {
				return err
			}
		}

		if dstTable.AutoIncrement != "" && dstDialect.Features().AutoincrMode == dialects.SequenceAutoincrMode {
			sqlstr, err := dstDialect.CreateSequenceSQL(ctx, engine.db, utils.SeqName(dstTableName))
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, sqlstr+";\n")
			if err != nil {
				return err
			}
		}

		sqlstr, _, err := dstDialect.CreateTableSQL(ctx, engine.db, dstTable, dstTableName)
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, sqlstr+";\n")
		if err != nil {
			return err
		}

		if len(dstTable.PKColumns()) > 0 && dstDialect.URI().DBType == schemas.MSSQL {
			fmt.Fprintf(w, "SET IDENTITY_INSERT [%s] ON;\n", dstTable.Name)
		}

		for _, index := range dstTable.Indexes {
			_, err = io.WriteString(w, dstDialect.CreateIndexSQL(dstTable.Name, index)+";\n")
			if err != nil {
				return err
			}
		}

		cols := table.ColumnsSeq()
		dstCols := dstTable.ColumnsSeq()

		colNames := engine.dialect.Quoter().Join(cols, ", ")
		destColNames := dstDialect.Quoter().Join(dstCols, ", ")

		rows, err := engine.DB().QueryContext(engine.defaultContext, "SELECT "+colNames+" FROM "+engine.Quote(originalTableName))
		if err != nil {
			return err
		}
		defer rows.Close()

		types, err := rows.ColumnTypes()
		if err != nil {
			return err
		}

		fields, err := rows.Columns()
		if err != nil {
			return err
		}

		sess := engine.NewSession()
		defer sess.Close()
		for rows.Next() {
			_, err = io.WriteString(w, "INSERT INTO "+quotedDstTableName+" ("+destColNames+") VALUES (")
			if err != nil {
				return err
			}

			scanResults, err := sess.engine.scanStringInterface(rows, fields, types)
			if err != nil {
				return err
			}
			for i, scanResult := range scanResults {
				stp := schemas.SQLType{Name: types[i].DatabaseTypeName()}
				s := scanResult.(*sql.NullString)
				if !s.Valid {
					if _, err = io.WriteString(w, "NULL"); err != nil {
						return err
					}
				} else {
					if table.Columns()[i].SQLType.IsBool() || stp.IsBool() || (dstDialect.URI().DBType == schemas.MSSQL && strings.EqualFold(stp.Name, schemas.Bit)) {
						val, err := strconv.ParseBool(s.String)
						if err != nil {
							return err
						}

						if _, err = io.WriteString(w, formatBool(val, dstDialect)); err != nil {
							return err
						}
					} else if stp.IsNumeric() {
						if _, err = io.WriteString(w, s.String); err != nil {
							return err
						}
					} else if sess.engine.dialect.URI().DBType == schemas.DAMENG && stp.IsTime() && len(s.String) == 25 {
						r := strings.ReplaceAll(s.String[:19], "T", " ")
						if _, err = io.WriteString(w, "'"+r+"'"); err != nil {
							return err
						}
					} else if len(s.String) == 0 {
						if _, err := io.WriteString(w, "''"); err != nil {
							return err
						}
					} else if dstDialect.URI().DBType == schemas.POSTGRES {
						if dstTable.Columns()[i].SQLType.IsBlob() {
							// Postgres has the escape format and we should use that for bytea data
							if _, err := fmt.Fprintf(w, "'\\x%x'", s.String); err != nil {
								return err
							}
						} else {
							// Postgres concatentates strings using || (NOTE: a NUL byte in a text segment will fail)
							toCheck := strings.ReplaceAll(s.String, "'", "''")
							for len(toCheck) > 0 {
								loc := controlCharactersRe.FindStringIndex(toCheck)
								if loc == nil {
									if _, err := io.WriteString(w, "'"+toCheck+"'"); err != nil {
										return err
									}
									break
								}
								if loc[0] > 0 {
									if _, err := io.WriteString(w, "'"+toCheck[:loc[0]]+"' || "); err != nil {
										return err
									}
								}
								if _, err := io.WriteString(w, "e'"); err != nil {
									return err
								}
								for i := loc[0]; i < loc[1]; i++ {
									if _, err := fmt.Fprintf(w, "\\x%02x", toCheck[i]); err != nil {
										return err
									}
								}
								toCheck = toCheck[loc[1]:]
								if len(toCheck) > 0 {
									if _, err := io.WriteString(w, "' || "); err != nil {
										return err
									}
								} else {
									if _, err := io.WriteString(w, "'"); err != nil {
										return err
									}
								}
							}
						}
					} else if dstDialect.URI().DBType == schemas.MYSQL {
						loc := controlCharactersRe.FindStringIndex(s.String)
						if loc == nil {
							if _, err := io.WriteString(w, "'"+strings.ReplaceAll(s.String, "'", "''")+"'"); err != nil {
								return err
							}
						} else {
							if _, err := io.WriteString(w, "CONCAT("); err != nil {
								return err
							}
							toCheck := strings.ReplaceAll(s.String, "'", "''")
							for len(toCheck) > 0 {
								loc := controlCharactersRe.FindStringIndex(toCheck)
								if loc == nil {
									if _, err := io.WriteString(w, "'"+toCheck+"')"); err != nil {
										return err
									}
									break
								}
								if loc[0] > 0 {
									if _, err := io.WriteString(w, "'"+toCheck[:loc[0]]+"', "); err != nil {
										return err
									}
								}
								for i := loc[0]; i < loc[1]-1; i++ {
									if _, err := io.WriteString(w, "CHAR("+strconv.Itoa(int(toCheck[i]))+"), "); err != nil {
										return err
									}
								}
								char := toCheck[loc[1]-1]
								toCheck = toCheck[loc[1]:]
								if len(toCheck) > 0 {
									if _, err := io.WriteString(w, "CHAR("+strconv.Itoa(int(char))+"), "); err != nil {
										return err
									}
								} else {
									if _, err = io.WriteString(w, "CHAR("+strconv.Itoa(int(char))+"))"); err != nil {
										return err
									}
								}
							}
						}
					} else if dstDialect.URI().DBType == schemas.SQLITE {
						if dstTable.Columns()[i].SQLType.IsBlob() {
							// SQLite has its escape format
							if _, err := fmt.Fprintf(w, "X'%x'", s.String); err != nil {
								return err
							}
						} else {
							// SQLite concatentates strings using || (NOTE: a NUL byte in a text segment will fail)
							toCheck := strings.ReplaceAll(s.String, "'", "''")
							for len(toCheck) > 0 {
								loc := controlCharactersRe.FindStringIndex(toCheck)
								if loc == nil {
									if _, err := io.WriteString(w, "'"+toCheck+"'"); err != nil {
										return err
									}
									break
								}
								if loc[0] > 0 {
									if _, err := io.WriteString(w, "'"+toCheck[:loc[0]]+"' || "); err != nil {
										return err
									}
								}
								if _, err := fmt.Fprintf(w, "X'%x'", toCheck[loc[0]:loc[1]]); err != nil {
									return err
								}
								toCheck = toCheck[loc[1]:]
								if len(toCheck) > 0 {
									if _, err := io.WriteString(w, " || "); err != nil {
										return err
									}
								}
							}
						}
					} else if dstDialect.URI().DBType == schemas.DAMENG || dstDialect.URI().DBType == schemas.ORACLE {
						if dstTable.Columns()[i].SQLType.IsBlob() {
							// ORACLE/DAMENG uses HEXTORAW
							if _, err := fmt.Fprintf(w, "HEXTORAW('%x')", s.String); err != nil {
								return err
							}
						} else {
							// ORACLE/DAMENG concatentates strings in multiple ways but uses CHAR and has CONCAT
							// (NOTE: a NUL byte in a text segment will fail)
							if _, err := io.WriteString(w, "CONCAT("); err != nil {
								return err
							}
							toCheck := strings.ReplaceAll(s.String, "'", "''")
							for len(toCheck) > 0 {
								loc := controlCharactersRe.FindStringIndex(toCheck)
								if loc == nil {
									if _, err := io.WriteString(w, "'"+toCheck+"')"); err != nil {
										return err
									}
									break
								}
								if loc[0] > 0 {
									if _, err := io.WriteString(w, "'"+toCheck[:loc[0]]+"', "); err != nil {
										return err
									}
								}
								for i := loc[0]; i < loc[1]-1; i++ {
									if _, err := io.WriteString(w, "CHAR("+strconv.Itoa(int(toCheck[i]))+"), "); err != nil {
										return err
									}
								}
								char := toCheck[loc[1]-1]
								toCheck = toCheck[loc[1]:]
								if len(toCheck) > 0 {
									if _, err := io.WriteString(w, "CHAR("+strconv.Itoa(int(char))+"), "); err != nil {
										return err
									}
								} else {
									if _, err = io.WriteString(w, "CHAR("+strconv.Itoa(int(char))+"))"); err != nil {
										return err
									}
								}
							}
						}
					} else if dstDialect.URI().DBType == schemas.MSSQL {
						if dstTable.Columns()[i].SQLType.IsBlob() {
							// MSSQL uses CONVERT(VARBINARY(MAX), '0xDEADBEEF', 1)
							if _, err := fmt.Fprintf(w, "CONVERT(VARBINARY(MAX), '0x%x', 1)", s.String); err != nil {
								return err
							}
						} else {
							if _, err = io.WriteString(w, "N'"+strings.ReplaceAll(s.String, "'", "''")+"'"); err != nil {
								return err
							}
						}
					} else {
						if _, err = io.WriteString(w, "'"+strings.ReplaceAll(s.String, "'", "''")+"'"); err != nil {
							return err
						}
					}
				}
				if i < len(scanResults)-1 {
					if _, err = io.WriteString(w, ","); err != nil {
						return err
					}
				}
			}
			_, err = io.WriteString(w, ");\n")
			if err != nil {
				return err
			}
		}
		if rows.Err() != nil {
			return rows.Err()
		}

		// FIXME: Hack for postgres
		if dstDialect.URI().DBType == schemas.POSTGRES && table.AutoIncrColumn() != nil {
			_, err = io.WriteString(w, "SELECT setval('"+dstTableName+"_id_seq', COALESCE((SELECT MAX("+table.AutoIncrColumn().Name+") + 1 FROM "+dstDialect.Quoter().Quote(dstTableName)+"), 1), false);\n")
			if err != nil {
				return err
			}
		}
		// !datbeohbbh! if no error, manually close
		rows.Close()
		sess.Close()
	}
	return nil
}

// Cascade use cascade or not
func (engine *Engine) Cascade(trueOrFalse ...bool) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Cascade(trueOrFalse...)
}

// Where method provide a condition query
func (engine *Engine) Where(query interface{}, args ...interface{}) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Where(query, args...)
}

// ID method provoide a condition as (id) = ?
func (engine *Engine) ID(id interface{}) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.ID(id)
}

// Before apply before Processor, affected bean is passed to closure arg
func (engine *Engine) Before(closures func(interface{})) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Before(closures)
}

// After apply after insert Processor, affected bean is passed to closure arg
func (engine *Engine) After(closures func(interface{})) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.After(closures)
}

// Charset set charset when create table, only support mysql now
func (engine *Engine) Charset(charset string) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Charset(charset)
}

// StoreEngine set store engine when create table, only support mysql now
func (engine *Engine) StoreEngine(storeEngine string) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.StoreEngine(storeEngine)
}

// Distinct use for distinct columns. Caution: when you are using cache,
// distinct will not be cached because cache system need id,
// but distinct will not provide id
func (engine *Engine) Distinct(columns ...string) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Distinct(columns...)
}

// Select customerize your select columns or contents
func (engine *Engine) Select(str string) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Select(str)
}

// Cols only use the parameters as select or update columns
func (engine *Engine) Cols(columns ...string) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Cols(columns...)
}

// AllCols indicates that all columns should be use
func (engine *Engine) AllCols() *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.AllCols()
}

// MustCols specify some columns must use even if they are empty
func (engine *Engine) MustCols(columns ...string) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.MustCols(columns...)
}

// UseBool xorm automatically retrieve condition according struct, but
// if struct has bool field, it will ignore them. So use UseBool
// to tell system to do not ignore them.
// If no parameters, it will use all the bool field of struct, or
// it will use parameters's columns
func (engine *Engine) UseBool(columns ...string) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.UseBool(columns...)
}

// Omit only not use the parameters as select or update columns
func (engine *Engine) Omit(columns ...string) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Omit(columns...)
}

// Nullable set null when column is zero-value and nullable for update
func (engine *Engine) Nullable(columns ...string) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Nullable(columns...)
}

// In will generate "column IN (?, ?)"
func (engine *Engine) In(column string, args ...interface{}) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.In(column, args...)
}

// NotIn will generate "column NOT IN (?, ?)"
func (engine *Engine) NotIn(column string, args ...interface{}) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.NotIn(column, args...)
}

// Incr provides a update string like "column = column + ?"
func (engine *Engine) Incr(column string, arg ...interface{}) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Incr(column, arg...)
}

// Decr provides a update string like "column = column - ?"
func (engine *Engine) Decr(column string, arg ...interface{}) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Decr(column, arg...)
}

// SetExpr provides a update string like "column = {expression}"
func (engine *Engine) SetExpr(column string, expression interface{}) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.SetExpr(column, expression)
}

// Table temporarily change the Get, Find, Update's table
func (engine *Engine) Table(tableNameOrBean interface{}) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Table(tableNameOrBean)
}

// Alias set the table alias
func (engine *Engine) Alias(alias string) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Alias(alias)
}

// Limit will generate "LIMIT start, limit"
func (engine *Engine) Limit(limit int, start ...int) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Limit(limit, start...)
}

// Desc will generate "ORDER BY column1 DESC, column2 DESC"
func (engine *Engine) Desc(colNames ...string) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Desc(colNames...)
}

// Asc will generate "ORDER BY column1,column2 Asc"
// This method can chainable use.
//
//	engine.Desc("name").Asc("age").Find(&users)
//	// SELECT * FROM user ORDER BY name DESC, age ASC
func (engine *Engine) Asc(colNames ...string) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Asc(colNames...)
}

// OrderBy will generate "ORDER BY order"
func (engine *Engine) OrderBy(order interface{}, args ...interface{}) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.OrderBy(order, args...)
}

// Prepare enables prepare statement
func (engine *Engine) Prepare() *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Prepare()
}

// Join the join_operator should be one of INNER, LEFT OUTER, CROSS etc - this will be prepended to JOIN
func (engine *Engine) Join(joinOperator string, tablename interface{}, condition interface{}, args ...interface{}) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Join(joinOperator, tablename, condition, args...)
}

// GroupBy generate group by statement
func (engine *Engine) GroupBy(keys string) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.GroupBy(keys)
}

// Having generate having statement
func (engine *Engine) Having(conditions string) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Having(conditions)
}

// DBVersion returns the database version
func (engine *Engine) DBVersion() (*schemas.Version, error) {
	return engine.dialect.Version(engine.defaultContext, engine.db)
}

// TableInfo get table info according to bean's content
func (engine *Engine) TableInfo(bean interface{}) (*schemas.Table, error) {
	v := utils.ReflectValue(bean)
	return engine.tagParser.ParseWithCache(v)
}

// IsTableEmpty if a table has any reocrd
func (engine *Engine) IsTableEmpty(bean interface{}) (bool, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.IsTableEmpty(bean)
}

// IsTableExist if a table is exist
func (engine *Engine) IsTableExist(beanOrTableName interface{}) (bool, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.IsTableExist(beanOrTableName)
}

// TableName returns table name with schema prefix if has
func (engine *Engine) TableName(bean interface{}, includeSchema ...bool) string {
	return dialects.FullTableName(engine.dialect, engine.GetTableMapper(), bean, includeSchema...)
}

// CreateIndexes create indexes
func (engine *Engine) CreateIndexes(bean interface{}) error {
	session := engine.NewSession()
	defer session.Close()
	return session.CreateIndexes(bean)
}

// CreateUniques create uniques
func (engine *Engine) CreateUniques(bean interface{}) error {
	session := engine.NewSession()
	defer session.Close()
	return session.CreateUniques(bean)
}

// ClearCacheBean if enabled cache, clear the cache bean
func (engine *Engine) ClearCacheBean(bean interface{}, id string) error {
	tableName := dialects.FullTableName(engine.dialect, engine.GetTableMapper(), bean)
	cacher := engine.GetCacher(tableName)
	if cacher != nil {
		cacher.ClearIds(tableName)
		cacher.DelBean(tableName, id)
	}
	return nil
}

// ClearCache if enabled cache, clear some tables' cache
func (engine *Engine) ClearCache(beans ...interface{}) error {
	for _, bean := range beans {
		tableName := dialects.FullTableName(engine.dialect, engine.GetTableMapper(), bean)
		cacher := engine.GetCacher(tableName)
		if cacher != nil {
			cacher.ClearIds(tableName)
			cacher.ClearBeans(tableName)
		}
	}
	return nil
}

// UnMapType remove table from tables cache
func (engine *Engine) UnMapType(t reflect.Type) {
	engine.tagParser.ClearCacheTable(t)
}

// CreateTables create tabls according bean
func (engine *Engine) CreateTables(beans ...interface{}) error {
	session := engine.NewSession()
	defer session.Close()

	err := session.Begin()
	if err != nil {
		return err
	}

	for _, bean := range beans {
		err = session.createTable(bean)
		if err != nil {
			_ = session.Rollback()
			return err
		}
	}
	return session.Commit()
}

// DropTables drop specify tables
func (engine *Engine) DropTables(beans ...interface{}) error {
	session := engine.NewSession()
	defer session.Close()

	err := session.Begin()
	if err != nil {
		return err
	}

	for _, bean := range beans {
		err = session.dropTable(bean)
		if err != nil {
			_ = session.Rollback()
			return err
		}
	}
	return session.Commit()
}

// DropIndexes drop indexes of a table
func (engine *Engine) DropIndexes(bean interface{}) error {
	session := engine.NewSession()
	defer session.Close()
	return session.DropIndexes(bean)
}

// Exec raw sql
func (engine *Engine) Exec(sqlOrArgs ...interface{}) (sql.Result, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.Exec(sqlOrArgs...)
}

// Query a raw sql and return records as []map[string][]byte
func (engine *Engine) Query(sqlOrArgs ...interface{}) (resultsSlice []map[string][]byte, err error) {
	session := engine.NewSession()
	defer session.Close()
	return session.Query(sqlOrArgs...)
}

// QueryString runs a raw sql and return records as []map[string]string
func (engine *Engine) QueryString(sqlOrArgs ...interface{}) ([]map[string]string, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.QueryString(sqlOrArgs...)
}

// QueryInterface runs a raw sql and return records as []map[string]interface{}
func (engine *Engine) QueryInterface(sqlOrArgs ...interface{}) ([]map[string]interface{}, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.QueryInterface(sqlOrArgs...)
}

// Insert one or more records
func (engine *Engine) Insert(beans ...interface{}) (int64, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.Insert(beans...)
}

// InsertOne insert only one record
func (engine *Engine) InsertOne(bean interface{}) (int64, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.Insert(bean)
}

// Update records, bean's non-empty fields are updated contents,
// condiBean' non-empty filds are conditions
// CAUTION:
//
//	1.bool will defaultly be updated content nor conditions
//	 You should call UseBool if you have bool to use.
//	2.float32 & float64 may be not inexact as conditions
func (engine *Engine) Update(bean interface{}, condiBeans ...interface{}) (int64, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.Update(bean, condiBeans...)
}

// Delete records, bean's non-empty fields are conditions
// At least one condition must be set.
func (engine *Engine) Delete(beans ...interface{}) (int64, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.Delete(beans...)
}

// Truncate records, bean's non-empty fields are conditions
// In contrast to Delete this method allows deletes without conditions.
func (engine *Engine) Truncate(beans ...interface{}) (int64, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.Truncate(beans...)
}

// Get retrieve one record from table, bean's non-empty fields
// are conditions
func (engine *Engine) Get(beans ...interface{}) (bool, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.Get(beans...)
}

// Exist returns true if the record exist otherwise return false
func (engine *Engine) Exist(bean ...interface{}) (bool, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.Exist(bean...)
}

// Find retrieve records from table, condiBeans's non-empty fields
// are conditions. beans could be []Struct, []*Struct, map[int64]Struct
// map[int64]*Struct
func (engine *Engine) Find(beans interface{}, condiBeans ...interface{}) error {
	session := engine.NewSession()
	defer session.Close()
	return session.Find(beans, condiBeans...)
}

// FindAndCount find the results and also return the counts
func (engine *Engine) FindAndCount(rowsSlicePtr interface{}, condiBean ...interface{}) (int64, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.FindAndCount(rowsSlicePtr, condiBean...)
}

// Iterate record by record handle records from table, bean's non-empty fields
// are conditions.
func (engine *Engine) Iterate(bean interface{}, fun IterFunc) error {
	session := engine.NewSession()
	defer session.Close()
	return session.Iterate(bean, fun)
}

// Rows return sql.Rows compatible Rows obj, as a forward Iterator object for iterating record by record, bean's non-empty fields
// are conditions.
func (engine *Engine) Rows(bean interface{}) (*Rows, error) {
	session := engine.NewSession()
	return session.Rows(bean)
}

// Count counts the records. bean's non-empty fields are conditions.
func (engine *Engine) Count(bean ...interface{}) (int64, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.Count(bean...)
}

// Sum sum the records by some column. bean's non-empty fields are conditions.
func (engine *Engine) Sum(bean interface{}, colName string) (float64, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.Sum(bean, colName)
}

// SumInt sum the records by some column. bean's non-empty fields are conditions.
func (engine *Engine) SumInt(bean interface{}, colName string) (int64, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.SumInt(bean, colName)
}

// Sums sum the records by some columns. bean's non-empty fields are conditions.
func (engine *Engine) Sums(bean interface{}, colNames ...string) ([]float64, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.Sums(bean, colNames...)
}

// SumsInt like Sums but return slice of int64 instead of float64.
func (engine *Engine) SumsInt(bean interface{}, colNames ...string) ([]int64, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.SumsInt(bean, colNames...)
}

// ImportFile SQL DDL file
func (engine *Engine) ImportFile(ddlPath string) ([]sql.Result, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.ImportFile(ddlPath)
}

// Import SQL DDL from io.Reader
func (engine *Engine) Import(r io.Reader) ([]sql.Result, error) {
	session := engine.NewSession()
	defer session.Close()
	return session.Import(r)
}

// nowTime return current time
func (engine *Engine) nowTime(col *schemas.Column) (interface{}, time.Time, error) {
	t := time.Now()
	result, err := dialects.FormatColumnTime(engine.dialect, engine.DatabaseTZ, col, t)
	if err != nil {
		return nil, time.Time{}, err
	}
	return result, t.In(engine.TZLocation), nil
}

// GetColumnMapper returns the column name mapper
func (engine *Engine) GetColumnMapper() names.Mapper {
	return engine.tagParser.GetColumnMapper()
}

// GetTableMapper returns the table name mapper
func (engine *Engine) GetTableMapper() names.Mapper {
	return engine.tagParser.GetTableMapper()
}

// GetTZLocation returns time zone of the application
func (engine *Engine) GetTZLocation() *time.Location {
	return engine.TZLocation
}

// SetTZLocation sets time zone of the application
func (engine *Engine) SetTZLocation(tz *time.Location) {
	engine.TZLocation = tz
}

// GetTZDatabase returns time zone of the database
func (engine *Engine) GetTZDatabase() *time.Location {
	return engine.DatabaseTZ
}

// SetTZDatabase sets time zone of the database
func (engine *Engine) SetTZDatabase(tz *time.Location) {
	engine.DatabaseTZ = tz
}

// SetSchema sets the schema of database
func (engine *Engine) SetSchema(schema string) {
	engine.dialect.URI().SetSchema(schema)
}

// AddHook adds a context Hook
func (engine *Engine) AddHook(hook contexts.Hook) {
	engine.db.AddHook(hook)
}

// Unscoped always disable struct tag "deleted"
func (engine *Engine) Unscoped() *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Unscoped()
}

func (engine *Engine) tbNameWithSchema(v string) string {
	return dialects.TableNameWithSchema(engine.dialect, v)
}

// Context creates a session with the context
func (engine *Engine) Context(ctx context.Context) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	return session.Context(ctx)
}

// SetDefaultContext set the default context
func (engine *Engine) SetDefaultContext(ctx context.Context) {
	engine.defaultContext = ctx
}

// PingContext tests if database is alive
func (engine *Engine) PingContext(ctx context.Context) error {
	session := engine.NewSession()
	defer session.Close()
	return session.PingContext(ctx)
}

// Transaction Execute sql wrapped in a transaction(abbr as tx), tx will automatic commit if no errors occurred
func (engine *Engine) Transaction(f func(*Session) (interface{}, error)) (interface{}, error) {
	session := engine.NewSession()
	defer session.Close()

	if err := session.Begin(); err != nil {
		return nil, err
	}

	result, err := f(session)
	if err != nil {
		return result, err
	}

	if err := session.Commit(); err != nil {
		return result, err
	}

	return result, nil
}

func (engine *Engine) IndexHint(op, forType, indexerOrColName string) *Session {
	session := engine.NewSession()
	session.isAutoClose = true
	session.statement.LastError = session.statement.IndexHint(op, forType, indexerOrColName)
	return session
}
