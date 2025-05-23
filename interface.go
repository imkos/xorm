// Copyright 2017 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"context"
	"database/sql"
	"reflect"
	"time"

	"github.com/imkos/xorm/caches"
	"github.com/imkos/xorm/contexts"
	"github.com/imkos/xorm/dialects"
	"github.com/imkos/xorm/log"
	"github.com/imkos/xorm/names"
	"github.com/imkos/xorm/schemas"
)

// Interface defines the interface which Engine, EngineGroup and Session will implementate.
type Interface interface {
	AllCols() *Session
	Alias(alias string) *Session
	Asc(colNames ...string) *Session
	BufferSize(size int) *Session
	Cols(columns ...string) *Session
	Count(...interface{}) (int64, error)
	CreateIndexes(bean interface{}) error
	CreateUniques(bean interface{}) error
	Decr(column string, arg ...interface{}) *Session
	Desc(...string) *Session
	Delete(...interface{}) (int64, error)
	Truncate(...interface{}) (int64, error)
	Distinct(columns ...string) *Session
	DropIndexes(bean interface{}) error
	Exec(sqlOrArgs ...interface{}) (sql.Result, error)
	Exist(bean ...interface{}) (bool, error)
	Find(interface{}, ...interface{}) error
	FindAndCount(interface{}, ...interface{}) (int64, error)
	Get(...interface{}) (bool, error)
	GroupBy(keys string) *Session
	ID(interface{}) *Session
	In(string, ...interface{}) *Session
	Incr(column string, arg ...interface{}) *Session
	Insert(...interface{}) (int64, error)
	InsertOne(interface{}) (int64, error)
	IsTableEmpty(bean interface{}) (bool, error)
	IsTableExist(beanOrTableName interface{}) (bool, error)
	Iterate(interface{}, IterFunc) error
	Limit(int, ...int) *Session
	MustCols(columns ...string) *Session
	NoAutoCondition(...bool) *Session
	NotIn(string, ...interface{}) *Session
	Nullable(...string) *Session
	Join(joinOperator string, tablename interface{}, condition interface{}, args ...interface{}) *Session
	Omit(columns ...string) *Session
	OrderBy(order interface{}, args ...interface{}) *Session
	Ping() error
	Query(sqlOrArgs ...interface{}) (resultsSlice []map[string][]byte, err error)
	QueryInterface(sqlOrArgs ...interface{}) ([]map[string]interface{}, error)
	QueryString(sqlOrArgs ...interface{}) ([]map[string]string, error)
	Rows(bean interface{}) (*Rows, error)
	SetExpr(string, interface{}) *Session
	Select(string) *Session
	SQL(interface{}, ...interface{}) *Session
	Sum(bean interface{}, colName string) (float64, error)
	SumInt(bean interface{}, colName string) (int64, error)
	Sums(bean interface{}, colNames ...string) ([]float64, error)
	SumsInt(bean interface{}, colNames ...string) ([]int64, error)
	Table(tableNameOrBean interface{}) *Session
	Unscoped() *Session
	Update(bean interface{}, condiBeans ...interface{}) (int64, error)
	UseBool(...string) *Session
	Where(interface{}, ...interface{}) *Session
}

// EngineInterface defines the interface which Engine, EngineGroup will implementate.
type EngineInterface interface {
	Interface

	Before(func(interface{})) *Session
	Charset(charset string) *Session
	ClearCache(...interface{}) error
	Context(context.Context) *Session
	CreateTables(...interface{}) error
	DBMetas() ([]*schemas.Table, error)
	DBVersion() (*schemas.Version, error)
	Dialect() dialects.Dialect
	DriverName() string
	DropTables(...interface{}) error
	DumpAllToFile(fp string, tp ...schemas.DBType) error
	GetCacher(string) caches.Cacher
	GetColumnMapper() names.Mapper
	GetDefaultCacher() caches.Cacher
	GetTableMapper() names.Mapper
	GetTZDatabase() *time.Location
	GetTZLocation() *time.Location
	ImportFile(fp string) ([]sql.Result, error)
	MapCacher(interface{}, caches.Cacher) error
	NewSession() *Session
	NoAutoTime() *Session
	Prepare() *Session
	Quote(string) string
	SetCacher(string, caches.Cacher)
	SetConnMaxLifetime(time.Duration)
	SetColumnMapper(names.Mapper)
	SetTagIdentifier(string)
	SetDefaultCacher(caches.Cacher)
	SetLogger(logger interface{})
	SetLogLevel(log.LogLevel)
	SetMapper(names.Mapper)
	SetMaxOpenConns(int)
	SetMaxIdleConns(int)
	SetQuotePolicy(dialects.QuotePolicy)
	SetSchema(string)
	SetTableMapper(names.Mapper)
	SetTZDatabase(tz *time.Location)
	SetTZLocation(tz *time.Location)
	AddHook(hook contexts.Hook)
	ShowSQL(show ...bool)
	Sync(...interface{}) error
	Sync2(...interface{}) error
	SyncWithOptions(SyncOptions, ...interface{}) (*SyncResult, error)
	StoreEngine(storeEngine string) *Session
	TableInfo(bean interface{}) (*schemas.Table, error)
	TableName(interface{}, ...bool) string
	UnMapType(reflect.Type)
	EnableSessionID(bool)
}

var (
	_ Interface       = &Session{}
	_ EngineInterface = &Engine{}
	_ EngineInterface = &EngineGroup{}
)
