// Copyright 2015 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/imkos/xorm/core"
	"github.com/imkos/xorm/schemas"
)

var (
	oracleReservedWords = map[string]bool{
		"ACCESS":                    true,
		"ACCOUNT":                   true,
		"ACTIVATE":                  true,
		"ADD":                       true,
		"ADMIN":                     true,
		"ADVISE":                    true,
		"AFTER":                     true,
		"ALL":                       true,
		"ALL_ROWS":                  true,
		"ALLOCATE":                  true,
		"ALTER":                     true,
		"ANALYZE":                   true,
		"AND":                       true,
		"ANY":                       true,
		"ARCHIVE":                   true,
		"ARCHIVELOG":                true,
		"ARRAY":                     true,
		"AS":                        true,
		"ASC":                       true,
		"AT":                        true,
		"AUDIT":                     true,
		"AUTHENTICATED":             true,
		"AUTHORIZATION":             true,
		"AUTOEXTEND":                true,
		"AUTOMATIC":                 true,
		"BACKUP":                    true,
		"BECOME":                    true,
		"BEFORE":                    true,
		"BEGIN":                     true,
		"BETWEEN":                   true,
		"BFILE":                     true,
		"BITMAP":                    true,
		"BLOB":                      true,
		"BLOCK":                     true,
		"BODY":                      true,
		"BY":                        true,
		"CACHE":                     true,
		"CACHE_INSTANCES":           true,
		"CANCEL":                    true,
		"CASCADE":                   true,
		"CAST":                      true,
		"CFILE":                     true,
		"CHAINED":                   true,
		"CHANGE":                    true,
		"CHAR":                      true,
		"CHAR_CS":                   true,
		"CHARACTER":                 true,
		"CHECK":                     true,
		"CHECKPOINT":                true,
		"CHOOSE":                    true,
		"CHUNK":                     true,
		"CLEAR":                     true,
		"CLOB":                      true,
		"CLONE":                     true,
		"CLOSE":                     true,
		"CLOSE_CACHED_OPEN_CURSORS": true,
		"CLUSTER":                   true,
		"COALESCE":                  true,
		"COLUMN":                    true,
		"COLUMNS":                   true,
		"COMMENT":                   true,
		"COMMIT":                    true,
		"COMMITTED":                 true,
		"COMPATIBILITY":             true,
		"COMPILE":                   true,
		"COMPLETE":                  true,
		"COMPOSITE_LIMIT":           true,
		"COMPRESS":                  true,
		"COMPUTE":                   true,
		"CONNECT":                   true,
		"CONNECT_TIME":              true,
		"CONSTRAINT":                true,
		"CONSTRAINTS":               true,
		"CONTENTS":                  true,
		"CONTINUE":                  true,
		"CONTROLFILE":               true,
		"CONVERT":                   true,
		"COST":                      true,
		"CPU_PER_CALL":              true,
		"CPU_PER_SESSION":           true,
		"CREATE":                    true,
		"CURRENT":                   true,
		"CURRENT_SCHEMA":            true,
		"CURREN_USER":               true,
		"CURSOR":                    true,
		"CYCLE":                     true,
		"DANGLING":                  true,
		"DATABASE":                  true,
		"DATAFILE":                  true,
		"DATAFILES":                 true,
		"DATAOBJNO":                 true,
		"DATE":                      true,
		"DBA":                       true,
		"DBHIGH":                    true,
		"DBLOW":                     true,
		"DBMAC":                     true,
		"DEALLOCATE":                true,
		"DEBUG":                     true,
		"DEC":                       true,
		"DECIMAL":                   true,
		"DECLARE":                   true,
		"DEFAULT":                   true,
		"DEFERRABLE":                true,
		"DEFERRED":                  true,
		"DEGREE":                    true,
		"DELETE":                    true,
		"DEREF":                     true,
		"DESC":                      true,
		"DIRECTORY":                 true,
		"DISABLE":                   true,
		"DISCONNECT":                true,
		"DISMOUNT":                  true,
		"DISTINCT":                  true,
		"DISTRIBUTED":               true,
		"DML":                       true,
		"DOUBLE":                    true,
		"DROP":                      true,
		"DUMP":                      true,
		"EACH":                      true,
		"ELSE":                      true,
		"ENABLE":                    true,
		"END":                       true,
		"ENFORCE":                   true,
		"ENTRY":                     true,
		"ESCAPE":                    true,
		"EXCEPT":                    true,
		"EXCEPTIONS":                true,
		"EXCHANGE":                  true,
		"EXCLUDING":                 true,
		"EXCLUSIVE":                 true,
		"EXECUTE":                   true,
		"EXISTS":                    true,
		"EXPIRE":                    true,
		"EXPLAIN":                   true,
		"EXTENT":                    true,
		"EXTENTS":                   true,
		"EXTERNALLY":                true,
		"FAILED_LOGIN_ATTEMPTS":     true,
		"FALSE":                     true,
		"FAST":                      true,
		"FILE":                      true,
		"FIRST_ROWS":                true,
		"FLAGGER":                   true,
		"FLOAT":                     true,
		"FLOB":                      true,
		"FLUSH":                     true,
		"FOR":                       true,
		"FORCE":                     true,
		"FOREIGN":                   true,
		"FREELIST":                  true,
		"FREELISTS":                 true,
		"FROM":                      true,
		"FULL":                      true,
		"FUNCTION":                  true,
		"GLOBAL":                    true,
		"GLOBALLY":                  true,
		"GLOBAL_NAME":               true,
		"GRANT":                     true,
		"GROUP":                     true,
		"GROUPS":                    true,
		"HASH":                      true,
		"HASHKEYS":                  true,
		"HAVING":                    true,
		"HEADER":                    true,
		"HEAP":                      true,
		"IDENTIFIED":                true,
		"IDGENERATORS":              true,
		"IDLE_TIME":                 true,
		"IF":                        true,
		"IMMEDIATE":                 true,
		"IN":                        true,
		"INCLUDING":                 true,
		"INCREMENT":                 true,
		"INDEX":                     true,
		"INDEXED":                   true,
		"INDEXES":                   true,
		"INDICATOR":                 true,
		"IND_PARTITION":             true,
		"INITIAL":                   true,
		"INITIALLY":                 true,
		"INITRANS":                  true,
		"INSERT":                    true,
		"INSTANCE":                  true,
		"INSTANCES":                 true,
		"INSTEAD":                   true,
		"INT":                       true,
		"INTEGER":                   true,
		"INTERMEDIATE":              true,
		"INTERSECT":                 true,
		"INTO":                      true,
		"IS":                        true,
		"ISOLATION":                 true,
		"ISOLATION_LEVEL":           true,
		"KEEP":                      true,
		"KEY":                       true,
		"KILL":                      true,
		"LABEL":                     true,
		"LAYER":                     true,
		"LESS":                      true,
		"LEVEL":                     true,
		"LIBRARY":                   true,
		"LIKE":                      true,
		"LIMIT":                     true,
		"LINK":                      true,
		"LIST":                      true,
		"LOB":                       true,
		"LOCAL":                     true,
		"LOCK":                      true,
		"LOCKED":                    true,
		"LOG":                       true,
		"LOGFILE":                   true,
		"LOGGING":                   true,
		"LOGICAL_READS_PER_CALL":    true,
		"LOGICAL_READS_PER_SESSION": true,
		"LONG":                      true,
		"MANAGE":                    true,
		"MASTER":                    true,
		"MAX":                       true,
		"MAXARCHLOGS":               true,
		"MAXDATAFILES":              true,
		"MAXEXTENTS":                true,
		"MAXINSTANCES":              true,
		"MAXLOGFILES":               true,
		"MAXLOGHISTORY":             true,
		"MAXLOGMEMBERS":             true,
		"MAXSIZE":                   true,
		"MAXTRANS":                  true,
		"MAXVALUE":                  true,
		"MIN":                       true,
		"MEMBER":                    true,
		"MINIMUM":                   true,
		"MINEXTENTS":                true,
		"MINUS":                     true,
		"MINVALUE":                  true,
		"MLSLABEL":                  true,
		"MLS_LABEL_FORMAT":          true,
		"MODE":                      true,
		"MODIFY":                    true,
		"MOUNT":                     true,
		"MOVE":                      true,
		"MTS_DISPATCHERS":           true,
		"MULTISET":                  true,
		"NATIONAL":                  true,
		"NCHAR":                     true,
		"NCHAR_CS":                  true,
		"NCLOB":                     true,
		"NEEDED":                    true,
		"NESTED":                    true,
		"NETWORK":                   true,
		"NEW":                       true,
		"NEXT":                      true,
		"NOARCHIVELOG":              true,
		"NOAUDIT":                   true,
		"NOCACHE":                   true,
		"NOCOMPRESS":                true,
		"NOCYCLE":                   true,
		"NOFORCE":                   true,
		"NOLOGGING":                 true,
		"NOMAXVALUE":                true,
		"NOMINVALUE":                true,
		"NONE":                      true,
		"NOORDER":                   true,
		"NOOVERRIDE":                true,
		"NOPARALLEL":                true,
		"NOREVERSE":                 true,
		"NORMAL":                    true,
		"NOSORT":                    true,
		"NOT":                       true,
		"NOTHING":                   true,
		"NOWAIT":                    true,
		"NULL":                      true,
		"NUMBER":                    true,
		"NUMERIC":                   true,
		"NVARCHAR2":                 true,
		"OBJECT":                    true,
		"OBJNO":                     true,
		"OBJNO_REUSE":               true,
		"OF":                        true,
		"OFF":                       true,
		"OFFLINE":                   true,
		"OID":                       true,
		"OIDINDEX":                  true,
		"OLD":                       true,
		"ON":                        true,
		"ONLINE":                    true,
		"ONLY":                      true,
		"OPCODE":                    true,
		"OPEN":                      true,
		"OPTIMAL":                   true,
		"OPTIMIZER_GOAL":            true,
		"OPTION":                    true,
		"OR":                        true,
		"ORDER":                     true,
		"ORGANIZATION":              true,
		"OSLABEL":                   true,
		"OVERFLOW":                  true,
		"OWN":                       true,
		"PACKAGE":                   true,
		"PARALLEL":                  true,
		"PARTITION":                 true,
		"PASSWORD":                  true,
		"PASSWORD_GRACE_TIME":       true,
		"PASSWORD_LIFE_TIME":        true,
		"PASSWORD_LOCK_TIME":        true,
		"PASSWORD_REUSE_MAX":        true,
		"PASSWORD_REUSE_TIME":       true,
		"PASSWORD_VERIFY_FUNCTION":  true,
		"PCTFREE":                   true,
		"PCTINCREASE":               true,
		"PCTTHRESHOLD":              true,
		"PCTUSED":                   true,
		"PCTVERSION":                true,
		"PERCENT":                   true,
		"PERMANENT":                 true,
		"PLAN":                      true,
		"PLSQL_DEBUG":               true,
		"POST_TRANSACTION":          true,
		"PRECISION":                 true,
		"PRESERVE":                  true,
		"PRIMARY":                   true,
		"PRIOR":                     true,
		"PRIVATE":                   true,
		"PRIVATE_SGA":               true,
		"PRIVILEGE":                 true,
		"PRIVILEGES":                true,
		"PROCEDURE":                 true,
		"PROFILE":                   true,
		"PUBLIC":                    true,
		"PURGE":                     true,
		"QUEUE":                     true,
		"QUOTA":                     true,
		"RANGE":                     true,
		"RAW":                       true,
		"RBA":                       true,
		"READ":                      true,
		"READUP":                    true,
		"REAL":                      true,
		"REBUILD":                   true,
		"RECOVER":                   true,
		"RECOVERABLE":               true,
		"RECOVERY":                  true,
		"REF":                       true,
		"REFERENCES":                true,
		"REFERENCING":               true,
		"REFRESH":                   true,
		"RENAME":                    true,
		"REPLACE":                   true,
		"RESET":                     true,
		"RESETLOGS":                 true,
		"RESIZE":                    true,
		"RESOURCE":                  true,
		"RESTRICTED":                true,
		"RETURN":                    true,
		"RETURNING":                 true,
		"REUSE":                     true,
		"REVERSE":                   true,
		"REVOKE":                    true,
		"ROLE":                      true,
		"ROLES":                     true,
		"ROLLBACK":                  true,
		"ROW":                       true,
		"ROWID":                     true,
		"ROWNUM":                    true,
		"ROWS":                      true,
		"RULE":                      true,
		"SAMPLE":                    true,
		"SAVEPOINT":                 true,
		"SB4":                       true,
		"SCAN_INSTANCES":            true,
		"SCHEMA":                    true,
		"SCN":                       true,
		"SCOPE":                     true,
		"SD_ALL":                    true,
		"SD_INHIBIT":                true,
		"SD_SHOW":                   true,
		"SEGMENT":                   true,
		"SEG_BLOCK":                 true,
		"SEG_FILE":                  true,
		"SELECT":                    true,
		"SEQUENCE":                  true,
		"SERIALIZABLE":              true,
		"SESSION":                   true,
		"SESSION_CACHED_CURSORS":    true,
		"SESSIONS_PER_USER":         true,
		"SET":                       true,
		"SHARE":                     true,
		"SHARED":                    true,
		"SHARED_POOL":               true,
		"SHRINK":                    true,
		"SIZE":                      true,
		"SKIP":                      true,
		"SKIP_UNUSABLE_INDEXES":     true,
		"SMALLINT":                  true,
		"SNAPSHOT":                  true,
		"SOME":                      true,
		"SORT":                      true,
		"SPECIFICATION":             true,
		"SPLIT":                     true,
		"SQL_TRACE":                 true,
		"STANDBY":                   true,
		"START":                     true,
		"STATEMENT_ID":              true,
		"STATISTICS":                true,
		"STOP":                      true,
		"STORAGE":                   true,
		"STORE":                     true,
		"STRUCTURE":                 true,
		"SUCCESSFUL":                true,
		"SWITCH":                    true,
		"SYS_OP_ENFORCE_NOT_NULL$":  true,
		"SYS_OP_NTCIMG$":            true,
		"SYNONYM":                   true,
		"SYSDATE":                   true,
		"SYSDBA":                    true,
		"SYSOPER":                   true,
		"SYSTEM":                    true,
		"TABLE":                     true,
		"TABLES":                    true,
		"TABLESPACE":                true,
		"TABLESPACE_NO":             true,
		"TABNO":                     true,
		"TEMPORARY":                 true,
		"THAN":                      true,
		"THE":                       true,
		"THEN":                      true,
		"THREAD":                    true,
		"TIMESTAMP":                 true,
		"TIME":                      true,
		"TO":                        true,
		"TOPLEVEL":                  true,
		"TRACE":                     true,
		"TRACING":                   true,
		"TRANSACTION":               true,
		"TRANSITIONAL":              true,
		"TRIGGER":                   true,
		"TRIGGERS":                  true,
		"TRUE":                      true,
		"TRUNCATE":                  true,
		"TX":                        true,
		"TYPE":                      true,
		"UB2":                       true,
		"UBA":                       true,
		"UID":                       true,
		"UNARCHIVED":                true,
		"UNDO":                      true,
		"UNION":                     true,
		"UNIQUE":                    true,
		"UNLIMITED":                 true,
		"UNLOCK":                    true,
		"UNRECOVERABLE":             true,
		"UNTIL":                     true,
		"UNUSABLE":                  true,
		"UNUSED":                    true,
		"UPDATABLE":                 true,
		"UPDATE":                    true,
		"USAGE":                     true,
		"USE":                       true,
		"USER":                      true,
		"USING":                     true,
		"VALIDATE":                  true,
		"VALIDATION":                true,
		"VALUE":                     true,
		"VALUES":                    true,
		"VARCHAR":                   true,
		"VARCHAR2":                  true,
		"VARYING":                   true,
		"VIEW":                      true,
		"WHEN":                      true,
		"WHENEVER":                  true,
		"WHERE":                     true,
		"WITH":                      true,
		"WITHOUT":                   true,
		"WORK":                      true,
		"WRITE":                     true,
		"WRITEDOWN":                 true,
		"WRITEUP":                   true,
		"XID":                       true,
		"YEAR":                      true,
		"ZONE":                      true,
	}

	oracleQuoter = schemas.Quoter{
		Prefix:     '"',
		Suffix:     '"',
		IsReserved: schemas.AlwaysReserve,
	}
)

type oracle struct {
	Base
	useLegacy bool
}

func (db *oracle) Init(uri *URI) error {
	db.quoter = oracleQuoter
	return db.Base.Init(db, uri)
}

func (db *oracle) UseLegacyLimitOffset() bool { return db.useLegacy }

func (db *oracle) SetParams(params map[string]string) {
	useLegacy, ok := params["USE_LEGACY_LIMIT_OFFSET"]
	if ok {
		if b, _ := strconv.ParseBool(useLegacy); b {
			db.useLegacy = true
		}
	}
}

func (db *oracle) Version(ctx context.Context, queryer core.Queryer) (*schemas.Version, error) {
	rows, err := queryer.QueryContext(ctx, "select * from v$version where banner like 'Oracle%'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var version string
	if !rows.Next() {
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		return nil, errors.New("unknow version")
	}

	if err := rows.Scan(&version); err != nil {
		return nil, err
	}
	return &schemas.Version{
		Number: version,
	}, nil
}

func (db *oracle) Features() *DialectFeatures {
	return &DialectFeatures{
		AutoincrMode: SequenceAutoincrMode,
	}
}

func (db *oracle) SQLType(c *schemas.Column) string {
	var res string
	switch t := c.SQLType.Name; t {
	case schemas.Bool:
		if c.Default == "true" {
			c.Default = "1"
		} else if c.Default == "false" {
			c.Default = "0"
		}
		res = "NUMBER(1,0)"
	case schemas.Bit, schemas.TinyInt, schemas.SmallInt, schemas.MediumInt, schemas.Int, schemas.Integer, schemas.BigInt, schemas.Serial, schemas.BigSerial:
		res = "NUMBER"
	case schemas.Binary, schemas.VarBinary, schemas.Blob, schemas.TinyBlob, schemas.MediumBlob, schemas.LongBlob, schemas.Bytea:
		return schemas.Blob
	case schemas.Time, schemas.DateTime, schemas.TimeStamp:
		res = schemas.TimeStamp
	case schemas.TimeStampz:
		res = "TIMESTAMP WITH TIME ZONE"
	case schemas.Float, schemas.Double, schemas.Numeric, schemas.Decimal:
		res = "NUMBER"
	case schemas.Text, schemas.MediumText, schemas.LongText, schemas.Json:
		res = "CLOB"
	case schemas.Char, schemas.Varchar, schemas.TinyText:
		res = "VARCHAR2"
	default:
		res = t
	}

	hasLen1 := c.Length > 0
	hasLen2 := c.Length2 > 0

	if hasLen2 {
		res += "(" + strconv.FormatInt(c.Length, 10) + "," + strconv.FormatInt(c.Length2, 10) + ")"
	} else if hasLen1 {
		res += "(" + strconv.FormatInt(c.Length, 10) + ")"
	}
	return res
}

func (db *oracle) ColumnTypeKind(t string) int {
	switch strings.ToUpper(t) {
	case "DATE":
		return schemas.TIME_TYPE
	case "CHAR", "NCHAR", "VARCHAR", "VARCHAR2", "NVARCHAR2", "LONG", "CLOB", "NCLOB":
		return schemas.TEXT_TYPE
	case "NUMBER":
		return schemas.NUMERIC_TYPE
	case "BLOB":
		return schemas.BLOB_TYPE
	default:
		return schemas.UNKNOW_TYPE
	}
}

func (db *oracle) AutoIncrStr() string {
	return "AUTO_INCREMENT"
}

func (db *oracle) IsReserved(name string) bool {
	_, ok := oracleReservedWords[strings.ToUpper(name)]
	return ok
}

func (db *oracle) DropTableSQL(tableName string) (string, bool) {
	return fmt.Sprintf("DROP TABLE \"%s\"", tableName), false
}

func (db *oracle) CreateTableSQL(ctx context.Context, queryer core.Queryer, table *schemas.Table, tableName string) (string, bool, error) {
	sql := "CREATE TABLE "
	if tableName == "" {
		tableName = table.Name
	}

	quoter := db.Quoter()
	sql += quoter.Quote(tableName) + " ("

	pkList := table.PrimaryKeys

	for _, colName := range table.ColumnsSeq() {
		col := table.GetColumn(colName)
		/*if col.IsPrimaryKey && len(pkList) == 1 {
			sql += col.String(b.dialect)
		} else {*/
		s, _ := ColumnString(db, col, false, false)
		sql += s
		// }
		sql = strings.TrimSpace(sql)
		sql += ", "
	}

	if len(pkList) > 0 {
		sql += "PRIMARY KEY ( "
		sql += quoter.Join(pkList, ",")
		sql += " ), "
	}

	sql = sql[:len(sql)-2] + ")"
	return sql, false, nil
}

func (db *oracle) IsSequenceExist(ctx context.Context, queryer core.Queryer, seqName string) (bool, error) {
	return db.HasRecords(queryer, ctx, `SELECT sequence_name FROM user_sequences WHERE sequence_name = :1`, seqName)
}

func (db *oracle) SetQuotePolicy(quotePolicy QuotePolicy) {
	switch quotePolicy {
	case QuotePolicyNone:
		q := oracleQuoter
		q.IsReserved = schemas.AlwaysNoReserve
		db.quoter = q
	case QuotePolicyReserved:
		q := oracleQuoter
		q.IsReserved = db.IsReserved
		db.quoter = q
	case QuotePolicyAlways:
		fallthrough
	default:
		db.quoter = oracleQuoter
	}
}

func (db *oracle) IndexCheckSQL(tableName, idxName string) (string, []interface{}) {
	args := []interface{}{tableName, idxName}
	return `SELECT INDEX_NAME FROM USER_INDEXES ` +
		`WHERE TABLE_NAME = :1 AND INDEX_NAME = :2`, args
}

func (db *oracle) DropIndexSQL(tableName string, index *schemas.Index) string {
	quote := db.dialect.Quoter().Quote
	var name string
	if index.IsRegular {
		name = index.XName(tableName)
	} else {
		name = index.Name
	}
	return fmt.Sprintf("DROP INDEX %v", quote(name))
}

func (db *oracle) IsTableExist(queryer core.Queryer, ctx context.Context, tableName string) (bool, error) {
	return db.HasRecords(queryer, ctx, `SELECT table_name FROM user_tables WHERE table_name = :1`, tableName)
}

func (db *oracle) IsColumnExist(queryer core.Queryer, ctx context.Context, tableName, colName string) (bool, error) {
	args := []interface{}{tableName, colName}
	query := "SELECT column_name FROM USER_TAB_COLUMNS WHERE table_name = :1" +
		" AND column_name = :2"
	return db.HasRecords(queryer, ctx, query, args...)
}

func (db *oracle) GetColumns(queryer core.Queryer, ctx context.Context, tableName string) ([]string, map[string]*schemas.Column, error) {
	args := []interface{}{tableName}
	s := "SELECT column_name,data_default,data_type,data_length,data_precision,data_scale," +
		"nullable FROM USER_TAB_COLUMNS WHERE table_name = :1"

	rows, err := queryer.QueryContext(ctx, s, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	cols := make(map[string]*schemas.Column)
	colSeq := make([]string, 0)
	for rows.Next() {
		col := new(schemas.Column)
		col.Indexes = make(map[string]int)

		var colName, colDefault, nullable, dataType, dataPrecision, dataScale *string
		var dataLen int64

		err = rows.Scan(&colName, &colDefault, &dataType, &dataLen, &dataPrecision,
			&dataScale, &nullable)
		if err != nil {
			return nil, nil, err
		}

		col.Name = strings.Trim(*colName, `" `)
		if colDefault != nil {
			col.Default = *colDefault
			col.DefaultIsEmpty = false
		}

		if *nullable == "Y" {
			col.Nullable = true
		} else {
			col.Nullable = false
		}

		var ignore bool

		var dt string
		var len1, len2 int64
		dts := strings.Split(*dataType, "(")
		dt = dts[0]
		if len(dts) > 1 {
			lens := strings.Split(dts[1][:len(dts[1])-1], ",")
			if len(lens) > 1 {
				len1, _ = strconv.ParseInt(lens[0], 10, 64)
				len2, _ = strconv.ParseInt(lens[1], 10, 64)
			} else {
				len1, _ = strconv.ParseInt(lens[0], 10, 64)
			}
		}

		switch dt {
		case "VARCHAR2":
			col.SQLType = schemas.SQLType{Name: schemas.Varchar, DefaultLength: len1, DefaultLength2: len2}
		case "NVARCHAR2":
			col.SQLType = schemas.SQLType{Name: schemas.NVarchar, DefaultLength: len1, DefaultLength2: len2}
		case "TIMESTAMP WITH TIME ZONE":
			col.SQLType = schemas.SQLType{Name: schemas.TimeStampz, DefaultLength: 0, DefaultLength2: 0}
		case "NUMBER":
			col.SQLType = schemas.SQLType{Name: schemas.Double, DefaultLength: len1, DefaultLength2: len2}
		case "LONG", "LONG RAW":
			col.SQLType = schemas.SQLType{Name: schemas.Text, DefaultLength: 0, DefaultLength2: 0}
		case "RAW":
			col.SQLType = schemas.SQLType{Name: schemas.Binary, DefaultLength: 0, DefaultLength2: 0}
		case "ROWID":
			col.SQLType = schemas.SQLType{Name: schemas.Varchar, DefaultLength: 18, DefaultLength2: 0}
		case "AQ$_SUBSCRIBERS":
			ignore = true
		default:
			col.SQLType = schemas.SQLType{Name: strings.ToUpper(dt), DefaultLength: len1, DefaultLength2: len2}
		}

		if ignore {
			continue
		}

		if _, ok := schemas.SqlTypes[col.SQLType.Name]; !ok {
			return nil, nil, fmt.Errorf("Unknown colType %v %v", *dataType, col.SQLType)
		}

		col.Length = dataLen

		if col.SQLType.IsText() || col.SQLType.IsTime() {
			if !col.DefaultIsEmpty {
				col.Default = "'" + col.Default + "'"
			}
		}
		cols[col.Name] = col
		colSeq = append(colSeq, col.Name)
	}
	if rows.Err() != nil {
		return nil, nil, rows.Err()
	}

	return colSeq, cols, nil
}

func (db *oracle) GetTables(queryer core.Queryer, ctx context.Context) ([]*schemas.Table, error) {
	args := []interface{}{}
	s := "SELECT table_name FROM user_tables"

	rows, err := queryer.QueryContext(ctx, s, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make([]*schemas.Table, 0)
	for rows.Next() {
		table := schemas.NewEmptyTable()
		err = rows.Scan(&table.Name)
		if err != nil {
			return nil, err
		}

		tables = append(tables, table)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return tables, nil
}

func (db *oracle) GetIndexes(queryer core.Queryer, ctx context.Context, tableName string) (map[string]*schemas.Index, error) {
	args := []interface{}{tableName}
	s := "SELECT t.column_name,i.uniqueness,i.index_name FROM user_ind_columns t,user_indexes i " +
		"WHERE t.index_name = i.index_name and t.table_name = i.table_name and t.table_name =:1"

	rows, err := queryer.QueryContext(ctx, s, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexes := make(map[string]*schemas.Index)
	for rows.Next() {
		var indexType int
		var indexName, colName, uniqueness string

		err = rows.Scan(&colName, &uniqueness, &indexName)
		if err != nil {
			return nil, err
		}

		indexName = strings.Trim(indexName, `" `)

		var isRegular bool
		if strings.HasPrefix(indexName, "IDX_"+tableName) || strings.HasPrefix(indexName, "UQE_"+tableName) {
			indexName = indexName[5+len(tableName):]
			isRegular = true
		}

		if uniqueness == "UNIQUE" {
			indexType = schemas.UniqueType
		} else {
			indexType = schemas.IndexType
		}

		var index *schemas.Index
		var ok bool
		if index, ok = indexes[indexName]; !ok {
			index = new(schemas.Index)
			index.Type = indexType
			index.Name = indexName
			index.IsRegular = isRegular
			indexes[indexName] = index
		}
		index.AddColumn(colName)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return indexes, nil
}

func (db *oracle) Filters() []Filter {
	return []Filter{
		&oracleSeqFilter{Prefix: ":", Start: 1},
	}
}

type godrorDriver struct {
	baseDriver
}

func (g *godrorDriver) Features() *DriverFeatures {
	return &DriverFeatures{
		SupportReturnInsertedID: false,
	}
}

func (g *godrorDriver) Parse(driverName, dataSourceName string) (*URI, error) {
	db := &URI{DBType: schemas.ORACLE}
	dsnPattern := regexp.MustCompile(
		`^(?:(?P<user>.*?)(?::(?P<passwd>.*))?@)?` + // [user[:password]@]
			`(?:(?P<net>[^\(]*)(?:\((?P<addr>[^\)]*)\))?)?` + // [net[(addr)]]
			`\/(?P<dbname>.*?)` + // /dbname
			`(?:\?(?P<params>[^\?]*))?$`) // [?param1=value1&paramN=valueN]
	matches := dsnPattern.FindStringSubmatch(dataSourceName)
	// tlsConfigRegister := make(map[string]*tls.Config)
	names := dsnPattern.SubexpNames()

	for i, match := range matches {
		if names[i] == "dbname" {
			db.DBName = match
		}
	}
	if db.DBName == "" {
		return nil, errors.New("dbname is empty")
	}
	return db, nil
}

func (g *godrorDriver) GenScanResult(colType string) (interface{}, error) {
	switch colType {
	case "CHAR", "NCHAR", "VARCHAR", "VARCHAR2", "NVARCHAR2", "LONG", "CLOB", "NCLOB":
		var s sql.NullString
		return &s, nil
	case "NUMBER":
		var s sql.NullString
		return &s, nil
	case "DATE":
		var s sql.NullTime
		return &s, nil
	case "BLOB":
		var r sql.RawBytes
		return &r, nil
	default:
		var r sql.RawBytes
		return &r, nil
	}
}

type oci8Driver struct {
	godrorDriver
}

// dataSourceName=user/password@ipv4:port/dbname
// dataSourceName=user/password@[ipv6]:port/dbname
func (o *oci8Driver) Parse(driverName, dataSourceName string) (*URI, error) {
	db := &URI{DBType: schemas.ORACLE}
	dsnPattern := regexp.MustCompile(
		`^(?P<user>.*)\/(?P<password>.*)@` + // user:password@
			`(?P<net>.*)` + // ip:port
			`\/(?P<dbname>.*)`) // dbname
	matches := dsnPattern.FindStringSubmatch(dataSourceName)
	names := dsnPattern.SubexpNames()
	for i, match := range matches {
		if names[i] == "dbname" {
			db.DBName = match
		}
	}
	if db.DBName == "" && len(matches) != 0 {
		return nil, errors.New("dbname is empty")
	}
	return db, nil
}

type oracleDriver struct {
	godrorDriver
}
