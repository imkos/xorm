// Copyright 2015 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

import (
	"strings"
	"time"

	"github.com/imkos/xorm/internal/utils"

	"github.com/imkos/xorm/schemas"
)

// FormatColumnTime format column time
func FormatColumnTime(dialect Dialect, dbLocation *time.Location, col *schemas.Column, t time.Time) (interface{}, error) {
	if utils.IsTimeZero(t) {
		if col.Nullable {
			return nil, nil
		}
		if col.SQLType.IsNumeric() {
			return 0, nil
		}
		if col.SQLType.Name == schemas.TimeStamp || col.SQLType.Name == schemas.TimeStampz {
			t = time.Unix(0, 0)
		}
	}

	tmZone := dbLocation
	if col.TimeZone != nil {
		tmZone = col.TimeZone
	}

	t = t.In(tmZone)

	switch col.SQLType.Name {
	case schemas.Date:
		return t.Format("2006-01-02"), nil
	case schemas.Time:
		layout := "15:04:05"
		if col.Length > 0 {
			// we can use int(...) casting here as it's very unlikely to a huge sized field
			layout += "." + strings.Repeat("0", int(col.Length))
		}
		return t.Format(layout), nil
	case schemas.DateTime, schemas.TimeStamp:
		layout := "2006-01-02 15:04:05"
		if col.Length > 0 {
			// we can use int(...) casting here as it's very unlikely to a huge sized field
			layout += "." + strings.Repeat("0", int(col.Length))
		}
		return t.Format(layout), nil
	case schemas.Varchar:
		return t.Format("2006-01-02 15:04:05"), nil
	case schemas.TimeStampz:
		if dialect.URI().DBType == schemas.MSSQL {
			return t.Format("2006-01-02T15:04:05.9999999Z07:00"), nil
		} else {
			return t.Format(time.RFC3339Nano), nil
		}
	case schemas.BigInt, schemas.Int:
		return t.Unix(), nil
	default:
		return t, nil
	}
}
