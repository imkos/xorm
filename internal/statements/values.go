// Copyright 2017 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package statements

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"math/big"
	"reflect"
	"time"

	"github.com/imkos/xorm/convert"
	"github.com/imkos/xorm/dialects"
	"github.com/imkos/xorm/internal/json"
	"github.com/imkos/xorm/schemas"
)

var (
	nullFloatType = reflect.TypeOf(sql.NullFloat64{})
	bigFloatType  = reflect.TypeOf(big.Float{})
)

// Value2Interface convert a field value of a struct to interface for putting into database
func (statement *Statement) Value2Interface(col *schemas.Column, fieldValue reflect.Value) (interface{}, error) {
	if fieldValue.CanAddr() {
		if fieldConvert, ok := fieldValue.Addr().Interface().(convert.Conversion); ok {
			data, err := fieldConvert.ToDB()
			if err != nil {
				return nil, err
			}
			if data == nil {
				if col.Nullable {
					return nil, nil
				}
				data = []byte{}
			}
			if col.SQLType.IsBlob() {
				return data, nil
			}
			return string(data), nil
		}
	}

	isNil := fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil()
	if !isNil {
		if fieldConvert, ok := fieldValue.Interface().(convert.Conversion); ok {
			data, err := fieldConvert.ToDB()
			if err != nil {
				return nil, err
			}
			if data == nil {
				if col.Nullable {
					return nil, nil
				}
				data = []byte{}
			}
			if col.SQLType.IsBlob() {
				return data, nil
			}
			return string(data), nil
		}
	}

	fieldType := fieldValue.Type()
	k := fieldType.Kind()
	if k == reflect.Ptr {
		if fieldValue.IsNil() {
			return nil, nil
		} else if !fieldValue.IsValid() {
			return nil, nil
		} else {
			// !nashtsai! deference pointer type to instance type
			fieldValue = fieldValue.Elem()
			fieldType = fieldValue.Type()
			k = fieldType.Kind()
		}
	}

	switch k {
	case reflect.Bool:
		return fieldValue.Bool(), nil
	case reflect.String:
		return fieldValue.String(), nil
	case reflect.Struct:
		if fieldType.ConvertibleTo(schemas.TimeType) {
			t := fieldValue.Convert(schemas.TimeType).Interface().(time.Time)
			tf, err := dialects.FormatColumnTime(statement.dialect, statement.defaultTimeZone, col, t)
			if val, ok := tf.(string); ok {
				var layout string
				switch col.SQLType.Name {
				case schemas.Date:
					layout = "yyyy-MM-dd"
				case schemas.Time:
					layout = "HH24:mi:ss"
				default:
					layout = "yyyy-MM-dd HH24:mi:ss"
				}
				return &DateTimeString{Layout: layout, Str: val}, err
			} else {
				return tf, err
			}
		} else if fieldType.ConvertibleTo(nullFloatType) {
			t := fieldValue.Convert(nullFloatType).Interface().(sql.NullFloat64)
			if !t.Valid {
				return nil, nil
			}
			return t.Float64, nil
		} else if fieldType.ConvertibleTo(bigFloatType) {
			t := fieldValue.Convert(bigFloatType).Interface().(big.Float)
			return t.String(), nil
		}

		if !col.IsJSON {
			// !<winxxp>! 增加支持driver.Valuer接口的结构，如sql.NullString
			if v, ok := fieldValue.Interface().(driver.Valuer); ok {
				return v.Value()
			}

			fieldTable, err := statement.tagParser.ParseWithCache(fieldValue)
			if err != nil {
				return nil, err
			}
			if len(fieldTable.PrimaryKeys) == 1 {
				pkField := reflect.Indirect(fieldValue).FieldByName(fieldTable.PKColumns()[0].FieldName)
				return pkField.Interface(), nil
			}
			return nil, fmt.Errorf("no primary key for col %v", col.Name)
		}

		if col.SQLType.IsText() {
			bytes, err := json.DefaultJSONHandler.Marshal(fieldValue.Interface())
			if err != nil {
				return nil, err
			}
			return string(bytes), nil
		} else if col.SQLType.IsBlob() {
			bytes, err := json.DefaultJSONHandler.Marshal(fieldValue.Interface())
			if err != nil {
				return nil, err
			}
			return bytes, nil
		}
		return nil, fmt.Errorf("Unsupported type %v", fieldValue.Type())
	case reflect.Complex64, reflect.Complex128:
		bytes, err := json.DefaultJSONHandler.Marshal(fieldValue.Interface())
		if err != nil {
			return nil, err
		}
		return string(bytes), nil
	case reflect.Array, reflect.Slice, reflect.Map:
		if !fieldValue.IsValid() {
			return fieldValue.Interface(), nil
		}

		if col.SQLType.IsText() {
			bytes, err := json.DefaultJSONHandler.Marshal(fieldValue.Interface())
			if err != nil {
				return nil, err
			}
			return string(bytes), nil
		} else if col.SQLType.IsBlob() {
			var bytes []byte
			var err error
			if (k == reflect.Slice) &&
				(fieldValue.Type().Elem().Kind() == reflect.Uint8) {
				bytes = fieldValue.Bytes()
			} else {
				bytes, err = json.DefaultJSONHandler.Marshal(fieldValue.Interface())
				if err != nil {
					return nil, err
				}
			}
			return bytes, nil
		}
		return nil, ErrUnSupportedType
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		return fieldValue.Uint(), nil
	default:
		return fieldValue.Interface(), nil
	}
}
