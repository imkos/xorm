// Copyright 2016 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/imkos/xorm/convert"
	"github.com/imkos/xorm/dialects"
	"github.com/imkos/xorm/internal/utils"
	"github.com/imkos/xorm/schemas"
	"xorm.io/builder"
)

// ErrNoElementsOnSlice represents an error there is no element when insert
var ErrNoElementsOnSlice = errors.New("no element on slice when insert")

// Insert insert one or more beans
func (session *Session) Insert(beans ...interface{}) (int64, error) {
	var affected int64
	var err error

	if session.isAutoClose {
		defer session.Close()
	}

	session.autoResetStatement = false
	defer func() {
		session.autoResetStatement = true
		session.resetStatement()
	}()

	for _, bean := range beans {
		var cnt int64
		var err error
		switch v := bean.(type) {
		case map[string]interface{}:
			cnt, err = session.insertMapInterface(v)
		case []map[string]interface{}:
			cnt, err = session.insertMultipleMapInterface(v)
		case map[string]string:
			cnt, err = session.insertMapString(v)
		case []map[string]string:
			cnt, err = session.insertMultipleMapString(v)
		default:
			sliceValue := reflect.Indirect(reflect.ValueOf(bean))
			if sliceValue.Kind() == reflect.Slice {
				cnt, err = session.insertMultipleStruct(bean)
			} else {
				cnt, err = session.insertStruct(bean)
			}
		}
		if err != nil {
			return affected, err
		}
		affected += cnt
	}

	return affected, err
}

func (session *Session) insertMultipleStruct(rowsSlicePtr interface{}) (int64, error) {
	sliceValue := reflect.Indirect(reflect.ValueOf(rowsSlicePtr))
	if sliceValue.Kind() != reflect.Slice {
		return 0, errors.New("needs a pointer to a slice")
	}

	if sliceValue.Len() <= 0 {
		return 0, ErrNoElementsOnSlice
	}

	if err := session.statement.SetRefBean(sliceValue.Index(0).Interface()); err != nil {
		return 0, err
	}

	tableName := session.statement.TableName()
	if len(tableName) == 0 {
		return 0, ErrTableNotFound
	}

	var (
		table          = session.statement.RefTable
		size           = sliceValue.Len()
		colNames       []string
		colMultiPlaces []string
		args           []interface{}
	)

	for i := 0; i < size; i++ {
		v := sliceValue.Index(i)
		var vv reflect.Value
		switch v.Kind() {
		case reflect.Interface:
			vv = reflect.Indirect(v.Elem())
		default:
			vv = reflect.Indirect(v)
		}
		elemValue := v.Interface()
		var colPlaces []string

		// handle BeforeInsertProcessor
		// !nashtsai! does user expect it's same slice to passed closure when using Before()/After() when insert multi??
		for _, closure := range session.beforeClosures {
			closure(elemValue)
		}

		if processor, ok := interface{}(elemValue).(BeforeInsertProcessor); ok {
			processor.BeforeInsert()
		}
		// --

		for _, col := range table.Columns() {
			ptrFieldValue, err := col.ValueOfV(&vv)
			if err != nil {
				return 0, err
			}
			fieldValue := *ptrFieldValue
			if col.IsAutoIncrement && utils.IsZero(fieldValue.Interface()) {
				if session.engine.dialect.Features().AutoincrMode == dialects.SequenceAutoincrMode {
					if i == 0 {
						colNames = append(colNames, col.Name)
					}
					colPlaces = append(colPlaces, utils.SeqName(tableName)+".nextval")
				}
				continue
			}
			if col.MapType == schemas.ONLYFROMDB {
				continue
			}
			if col.IsDeleted {
				continue
			}
			if session.statement.OmitColumnMap.Contain(col.Name) {
				continue
			}
			if len(session.statement.ColumnMap) > 0 && !session.statement.ColumnMap.Contain(col.Name) {
				continue
			}
			// !satorunooshie! set fieldValue as nil when column is nullable and zero-value
			if _, ok := getFlagForColumn(session.statement.NullableMap, col); ok {
				if col.Nullable && utils.IsValueZero(fieldValue) {
					var nilValue *int
					fieldValue = reflect.ValueOf(nilValue)
				}
			}
			if (col.IsCreated || col.IsUpdated) && session.statement.UseAutoTime {
				val, t, err := session.engine.nowTime(col)
				if err != nil {
					return 0, err
				}
				args = append(args, val)

				colName := col.Name
				session.afterClosures = append(session.afterClosures, func(bean interface{}) {
					col := table.GetColumn(colName)
					setColumnTime(bean, col, t)
				})
			} else if col.IsVersion && session.statement.CheckVersion {
				args = append(args, 1)
				colName := col.Name
				session.afterClosures = append(session.afterClosures, func(bean interface{}) {
					col := table.GetColumn(colName)
					setColumnInt(bean, col, 1)
				})
			} else {
				arg, err := session.statement.Value2Interface(col, fieldValue)
				if err != nil {
					return 0, err
				}
				args = append(args, arg)
			}

			if i == 0 {
				colNames = append(colNames, col.Name)
			}
			colPlaces = append(colPlaces, "?")
		}

		colMultiPlaces = append(colMultiPlaces, strings.Join(colPlaces, ", "))
	}
	cleanupProcessorsClosures(&session.beforeClosures)

	w := builder.NewWriter()
	if err := session.statement.WriteInsertMultiple(w, tableName, colNames, colMultiPlaces); err != nil {
		return 0, err
	}

	res, err := session.exec(w.String(), args...)
	if err != nil {
		return 0, err
	}

	_ = session.cacheInsert(tableName)

	lenAfterClosures := len(session.afterClosures)
	for i := 0; i < size; i++ {
		elemValue := reflect.Indirect(sliceValue.Index(i)).Addr().Interface()

		// handle AfterInsertProcessor
		if session.isAutoCommit {
			// !nashtsai! does user expect it's same slice to passed closure when using Before()/After() when insert multi??
			for _, closure := range session.afterClosures {
				closure(elemValue)
			}
			if processor, ok := elemValue.(AfterInsertProcessor); ok {
				processor.AfterInsert()
			}
		} else {
			if lenAfterClosures > 0 {
				if value, has := session.afterInsertBeans[elemValue]; has && value != nil {
					*value = append(*value, session.afterClosures...)
				} else {
					afterClosures := make([]func(interface{}), lenAfterClosures)
					copy(afterClosures, session.afterClosures)
					session.afterInsertBeans[elemValue] = &afterClosures
				}
			} else {
				if _, ok := elemValue.(AfterInsertProcessor); ok {
					session.afterInsertBeans[elemValue] = nil
				}
			}
		}
	}

	cleanupProcessorsClosures(&session.afterClosures)
	return res.RowsAffected()
}

// InsertMulti insert multiple records
func (session *Session) InsertMulti(rowsSlicePtr interface{}) (int64, error) {
	if session.isAutoClose {
		defer session.Close()
	}

	sliceValue := reflect.Indirect(reflect.ValueOf(rowsSlicePtr))
	if sliceValue.Kind() != reflect.Slice {
		return 0, ErrPtrSliceType
	}

	return session.insertMultipleStruct(rowsSlicePtr)
}

func (session *Session) insertStruct(bean interface{}) (int64, error) {
	if err := session.statement.SetRefBean(bean); err != nil {
		return 0, err
	}
	if len(session.statement.TableName()) == 0 {
		return 0, ErrTableNotFound
	}

	// handle BeforeInsertProcessor
	for _, closure := range session.beforeClosures {
		closure(bean)
	}
	cleanupProcessorsClosures(&session.beforeClosures) // cleanup after used

	if processor, ok := interface{}(bean).(BeforeInsertProcessor); ok {
		processor.BeforeInsert()
	}

	tableName := session.statement.TableName()
	table := session.statement.RefTable

	colNames, args, err := session.genInsertColumns(bean)
	if err != nil {
		return 0, err
	}

	sqlStr, args, err := session.statement.GenInsertSQL(colNames, args)
	if err != nil {
		return 0, err
	}
	sqlStr = session.engine.dialect.Quoter().Replace(sqlStr)

	handleAfterInsertProcessorFunc := func(bean interface{}) {
		if session.isAutoCommit {
			for _, closure := range session.afterClosures {
				closure(bean)
			}
			if processor, ok := interface{}(bean).(AfterInsertProcessor); ok {
				processor.AfterInsert()
			}
		} else {
			lenAfterClosures := len(session.afterClosures)
			if lenAfterClosures > 0 {
				if value, has := session.afterInsertBeans[bean]; has && value != nil {
					*value = append(*value, session.afterClosures...)
				} else {
					afterClosures := make([]func(interface{}), lenAfterClosures)
					copy(afterClosures, session.afterClosures)
					session.afterInsertBeans[bean] = &afterClosures
				}
			} else {
				if _, ok := interface{}(bean).(AfterInsertProcessor); ok {
					session.afterInsertBeans[bean] = nil
				}
			}
		}
		cleanupProcessorsClosures(&session.afterClosures) // cleanup after used
	}

	// if there is auto increment column and driver don't support return it
	if len(table.AutoIncrement) > 0 && !session.engine.driver.Features().SupportReturnInsertedID {
		var sql string
		var newArgs []interface{}
		var needCommit bool
		var id int64
		if session.engine.dialect.URI().DBType == schemas.ORACLE || session.engine.dialect.URI().DBType == schemas.DAMENG {
			if session.isAutoCommit { // if it's not in transaction
				if err := session.Begin(); err != nil {
					return 0, err
				}
				needCommit = true
			}
			_, err := session.exec(sqlStr, args...)
			if err != nil {
				return 0, err
			}
			i := utils.IndexSlice(colNames, table.AutoIncrement)
			if i > -1 {
				id, err = convert.AsInt64(args[i])
				if err != nil {
					return 0, err
				}
			} else {
				sql = fmt.Sprintf("select %s.currval from dual", utils.SeqName(tableName))
			}
		} else {
			sql = sqlStr
			newArgs = args
		}

		if id == 0 {
			err := session.queryRow(sql, newArgs...).Scan(&id)
			if err != nil {
				return 0, err
			}
		}
		if needCommit {
			if err := session.Commit(); err != nil {
				return 0, err
			}
		}
		if id == 0 {
			return 0, errors.New("insert successfully but not returned id")
		}

		defer handleAfterInsertProcessorFunc(bean)

		_ = session.cacheInsert(tableName)

		if table.Version != "" && session.statement.CheckVersion {
			verValue, err := table.VersionColumn().ValueOf(bean)
			if err != nil {
				session.engine.logger.Errorf("%v", err)
			} else if verValue.IsValid() && verValue.CanSet() {
				session.incrVersionFieldValue(verValue)
			}
		}

		aiValue, err := table.AutoIncrColumn().ValueOf(bean)
		if err != nil {
			session.engine.logger.Errorf("%v", err)
		}

		if aiValue == nil || !aiValue.IsValid() || !aiValue.CanSet() {
			return 1, nil
		}

		return 1, convert.AssignValue(*aiValue, id)
	}

	res, err := session.exec(sqlStr, args...)
	if err != nil {
		return 0, err
	}

	defer handleAfterInsertProcessorFunc(bean)

	_ = session.cacheInsert(tableName)

	if table.Version != "" && session.statement.CheckVersion {
		verValue, err := table.VersionColumn().ValueOf(bean)
		if err != nil {
			session.engine.logger.Errorf("%v", err)
		} else if verValue.IsValid() && verValue.CanSet() {
			session.incrVersionFieldValue(verValue)
		}
	}

	if table.AutoIncrement == "" {
		return res.RowsAffected()
	}

	var id int64
	id, err = res.LastInsertId()
	if err != nil || id <= 0 {
		return res.RowsAffected()
	}

	aiValue, err := table.AutoIncrColumn().ValueOf(bean)
	if err != nil {
		session.engine.logger.Errorf("%v", err)
	}

	if aiValue == nil || !aiValue.IsValid() || !aiValue.CanSet() {
		return res.RowsAffected()
	}

	if err := convert.AssignValue(*aiValue, id); err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

// InsertOne insert only one struct into database as a record.
// The in parameter bean must a struct or a point to struct. The return
// parameter is inserted and error
// Deprecated: Please use Insert directly
func (session *Session) InsertOne(bean interface{}) (int64, error) {
	if session.isAutoClose {
		defer session.Close()
	}

	return session.insertStruct(bean)
}

func (session *Session) cacheInsert(table string) error {
	if !session.statement.UseCache {
		return nil
	}
	cacher := session.engine.cacherMgr.GetCacher(table)
	if cacher == nil {
		return nil
	}
	session.engine.logger.Debugf("[cache] clear SQL: %v", table)
	cacher.ClearIds(table)
	return nil
}

// genInsertColumns generates insert needed columns
func (session *Session) genInsertColumns(bean interface{}) ([]string, []interface{}, error) {
	table := session.statement.RefTable
	colNames := make([]string, 0, len(table.ColumnsSeq()))
	args := make([]interface{}, 0, len(table.ColumnsSeq()))

	for _, col := range table.Columns() {
		if col.MapType == schemas.ONLYFROMDB {
			continue
		}
		if session.statement.OmitColumnMap.Contain(col.Name) {
			continue
		}
		if len(session.statement.ColumnMap) > 0 && !session.statement.ColumnMap.Contain(col.Name) {
			continue
		}
		if session.statement.IncrColumns.IsColExist(col.Name) {
			continue
		} else if session.statement.DecrColumns.IsColExist(col.Name) {
			continue
		} else if session.statement.ExprColumns.IsColExist(col.Name) {
			continue
		}

		if col.IsDeleted {
			zeroTime := time.Date(1, 1, 1, 0, 0, 0, 0, session.engine.DatabaseTZ)
			arg, err := dialects.FormatColumnTime(session.engine.dialect, session.engine.DatabaseTZ, col, zeroTime)
			if err != nil {
				return nil, nil, err
			}
			args = append(args, arg)
			colNames = append(colNames, col.Name)
			continue
		}

		fieldValuePtr, err := col.ValueOf(bean)
		if err != nil {
			return nil, nil, err
		}
		fieldValue := *fieldValuePtr

		if col.IsAutoIncrement && utils.IsValueZero(fieldValue) {
			continue
		}

		// !evalphobia! set fieldValue as nil when column is nullable and zero-value
		if _, ok := getFlagForColumn(session.statement.NullableMap, col); ok {
			if col.Nullable && utils.IsValueZero(fieldValue) {
				var nilValue *int
				fieldValue = reflect.ValueOf(nilValue)
			}
		}

		if (col.IsCreated || col.IsUpdated) && session.statement.UseAutoTime /*&& isZero(fieldValue.Interface())*/ {
			// if time is non-empty, then set to auto time
			val, t, err := session.engine.nowTime(col)
			if err != nil {
				return nil, nil, err
			}
			args = append(args, val)

			colName := col.Name
			session.afterClosures = append(session.afterClosures, func(bean interface{}) {
				col := table.GetColumn(colName)
				setColumnTime(bean, col, t)
			})
		} else if col.IsVersion && session.statement.CheckVersion {
			args = append(args, 1)
		} else {
			arg, err := session.statement.Value2Interface(col, fieldValue)
			if err != nil {
				return colNames, args, err
			}
			args = append(args, arg)
		}

		colNames = append(colNames, col.Name)
	}
	return colNames, args, nil
}

func (session *Session) insertMapInterface(m map[string]interface{}) (int64, error) {
	if len(m) == 0 {
		return 0, ErrParamsType
	}

	tableName := session.statement.TableName()
	if len(tableName) == 0 {
		return 0, ErrTableNotFound
	}

	columns := make([]string, 0, len(m))
	exprs := session.statement.ExprColumns
	for k := range m {
		if !exprs.IsColExist(k) {
			columns = append(columns, k)
		}
	}
	sort.Strings(columns)

	args := make([]interface{}, 0, len(m))
	for _, colName := range columns {
		args = append(args, m[colName])
	}

	return session.insertMap(columns, args)
}

func (session *Session) insertMultipleMapInterface(maps []map[string]interface{}) (int64, error) {
	if len(maps) == 0 {
		return 0, ErrNoElementsOnSlice
	}

	tableName := session.statement.TableName()
	if len(tableName) == 0 {
		return 0, ErrTableNotFound
	}

	columns := make([]string, 0, len(maps[0]))
	exprs := session.statement.ExprColumns
	for k := range maps[0] {
		if !exprs.IsColExist(k) {
			columns = append(columns, k)
		}
	}
	sort.Strings(columns)

	argss := make([][]interface{}, 0, len(maps))
	for _, m := range maps {
		args := make([]interface{}, 0, len(m))
		for _, colName := range columns {
			args = append(args, m[colName])
		}
		argss = append(argss, args)
	}

	return session.insertMultipleMap(columns, argss)
}

func (session *Session) insertMapString(m map[string]string) (int64, error) {
	if len(m) == 0 {
		return 0, ErrParamsType
	}

	tableName := session.statement.TableName()
	if len(tableName) == 0 {
		return 0, ErrTableNotFound
	}

	columns := make([]string, 0, len(m))
	exprs := session.statement.ExprColumns
	for k := range m {
		if !exprs.IsColExist(k) {
			columns = append(columns, k)
		}
	}

	sort.Strings(columns)

	args := make([]interface{}, 0, len(m))
	for _, colName := range columns {
		args = append(args, m[colName])
	}

	return session.insertMap(columns, args)
}

func (session *Session) insertMultipleMapString(maps []map[string]string) (int64, error) {
	if len(maps) == 0 {
		return 0, ErrNoElementsOnSlice
	}

	tableName := session.statement.TableName()
	if len(tableName) == 0 {
		return 0, ErrTableNotFound
	}

	columns := make([]string, 0, len(maps[0]))
	exprs := session.statement.ExprColumns
	for k := range maps[0] {
		if !exprs.IsColExist(k) {
			columns = append(columns, k)
		}
	}
	sort.Strings(columns)

	argss := make([][]interface{}, 0, len(maps))
	for _, m := range maps {
		args := make([]interface{}, 0, len(m))
		for _, colName := range columns {
			args = append(args, m[colName])
		}
		argss = append(argss, args)
	}

	return session.insertMultipleMap(columns, argss)
}

func (session *Session) insertMap(columns []string, args []interface{}) (int64, error) {
	tableName := session.statement.TableName()
	if len(tableName) == 0 {
		return 0, ErrTableNotFound
	}

	sql, args, err := session.statement.GenInsertMapSQL(columns, args)
	if err != nil {
		return 0, err
	}
	sql = session.engine.dialect.Quoter().Replace(sql)

	if err := session.cacheInsert(tableName); err != nil {
		return 0, err
	}

	res, err := session.exec(sql, args...)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return affected, nil
}

func (session *Session) insertMultipleMap(columns []string, argss [][]interface{}) (int64, error) {
	tableName := session.statement.TableName()
	if len(tableName) == 0 {
		return 0, ErrTableNotFound
	}

	sql, args, err := session.statement.GenInsertMultipleMapSQL(columns, argss)
	if err != nil {
		return 0, err
	}
	sql = session.engine.dialect.Quoter().Replace(sql)

	if err := session.cacheInsert(tableName); err != nil {
		return 0, err
	}

	res, err := session.exec(sql, args...)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return affected, nil
}
