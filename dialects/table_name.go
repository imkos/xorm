// Copyright 2015 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/imkos/xorm/internal/utils"
	"github.com/imkos/xorm/names"
	"github.com/imkos/xorm/schemas"
)

// TableNameWithSchema will add schema prefix on table name if possible
func TableNameWithSchema(dialect Dialect, tableName string) string {
	// Add schema name as prefix of table name.
	// Only for postgres database.
	if dialect.URI().Schema != "" && !strings.Contains(tableName, ".") {
		return fmt.Sprintf("%s.%s", dialect.URI().Schema, tableName)
	}
	return tableName
}

// TableNameNoSchema returns table name with given tableName
func TableNameNoSchema(dialect Dialect, mapper names.Mapper, tableName interface{}) string {
	quote := dialect.Quoter().Quote
	switch tt := tableName.(type) {
	case []string:
		if len(tt) > 1 {
			if dialect.URI().DBType == schemas.ORACLE {
				return fmt.Sprintf("%v %v", quote(tt[0]), quote(tt[1]))
			}
			return fmt.Sprintf("%v AS %v", quote(tt[0]), quote(tt[1]))
		} else if len(tt) == 1 {
			return quote(tt[0])
		}
	case []interface{}:
		l := len(tt)
		var table string
		if l > 0 {
			f := tt[0]
			switch f.(type) {
			case string:
				table = f.(string)
			case names.TableName:
				table = f.(names.TableName).TableName()
			default:
				v := utils.ReflectValue(f)
				t := v.Type()
				if t.Kind() == reflect.Struct {
					table = names.GetTableName(mapper, v)
				} else {
					table = quote(fmt.Sprintf("%v", f))
				}
			}
		}
		if l > 1 {
			if dialect.URI().DBType == schemas.ORACLE {
				return fmt.Sprintf("%v %v", quote(table), quote(fmt.Sprintf("%v", tt[1])))
			}
			return fmt.Sprintf("%v AS %v", quote(table), quote(fmt.Sprintf("%v", tt[1])))
		} else if l == 1 {
			return quote(table)
		}
	case names.TableName:
		return tableName.(names.TableName).TableName()
	case string:
		return tableName.(string)
	case reflect.Value:
		v := tableName.(reflect.Value)
		return names.GetTableName(mapper, v)
	default:
		v := utils.ReflectValue(tableName)
		t := v.Type()
		if t.Kind() == reflect.Struct {
			return names.GetTableName(mapper, v)
		}
		return quote(fmt.Sprintf("%v", tableName))
	}
	return ""
}

// FullTableName returns table name with quote and schema according parameter
func FullTableName(dialect Dialect, mapper names.Mapper, bean interface{}, includeSchema ...bool) string {
	tbName := TableNameNoSchema(dialect, mapper, bean)
	if len(includeSchema) > 0 && includeSchema[0] && !utils.IsSubQuery(tbName) {
		tbName = TableNameWithSchema(dialect, tbName)
	}
	return tbName
}
