# xorm

[中文](https://gitea.com/xorm/xorm/src/branch/v1/README_CN.md)

Xorm is a simple and powerful ORM for Go.

[![Build Status](https://gitea.com/xorm/xorm/actions/workflows/release-tag/badge.svg)](https://gitea.com/xorm/xorm/actions) [![](http://gocover.io/_badge/github.com/imkos/xorm)](https://gocover.io/github.com/imkos/xorm) [![](https://goreportcard.com/badge/github.com/imkos/xorm)](https://goreportcard.com/report/github.com/imkos/xorm) [![Join the chat at https://img.shields.io/discord/323460943201959939.svg](https://img.shields.io/discord/323460943201959939.svg)](https://discord.gg/HuR2CF3)

## Notice

v1.0.0 has some break changes from v0.8.2.

- Removed some non gonic function name `Id`, `Sql`, please use `ID`, `SQL` instead.
- Removed the dependent from `xorm.io/core` and moved the codes to `github.com/imkos/xorm/core`, `github.com/imkos/xorm/names`, `github.com/imkos/xorm/schemas` and others.
- Renamed some interface names. i.e. `core.IMapper` -> `names.Mapper`, `core.ILogger` -> `log.Logger`.

## Features

* Struct <-> Table Mapping Support
* Chainable APIs
* Transaction Support
* Both ORM and raw SQL operation Support
* Sync database schema Support
* Query Cache speed up
* Database Reverse support via [xorm.io/reverse](https://xorm.io/reverse)
* Simple cascade loading support
* Optimistic Locking support
* SQL Builder support via [xorm.io/builder](https://xorm.io/builder)
* Automatical Read/Write seperatelly
* Postgres schema support
* Context Cache support
* Support log/SQLLog context

## Drivers Support

Drivers for Go's sql package which currently support database/sql includes:

* [Mysql5.*](https://github.com/mysql/mysql-server/tree/5.7) / [Mysql8.*](https://github.com/mysql/mysql-server) / [Mariadb](https://github.com/MariaDB/server) / [Tidb](https://github.com/pingcap/tidb)
  - [github.com/go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)
  - [github.com/ziutek/mymysql/godrv](https://github.com/ziutek/mymysql/godrv)

* [Postgres](https://github.com/postgres/postgres) / [Cockroach](https://github.com/cockroachdb/cockroach)
  - [github.com/lib/pq](https://github.com/lib/pq)
  - [github.com/jackc/pgx](https://github.com/jackc/pgx)

* [SQLite](https://sqlite.org)
  - [github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3)
  - [modernc.org/sqlite](https://gitlab.com/cznic/sqlite)

* MsSql
  - [github.com/microsoft/go-mssqldb](https://github.com/microsoft/go-mssqldb)

* Oracle
  - [github.com/godror/godror](https://github.com/godror/godror) (experiment)
  - [github.com/mattn/go-oci8](https://github.com/mattn/go-oci8) (experiment)
  - [github.com/sijms/go-ora](https://github.com/sijms/go-ora) (experiment)

## Installation

	go get github.com/imkos/xorm

## Documents

* [Manual](http://xorm.io/docs)

* [GoDoc](http://pkg.go.dev/github.com/imkos/xorm)

## Quick Start

* Create Engine

Firstly, we should new an engine for a database.

```Go
engine, err := xorm.NewEngine(driverName, dataSourceName)
```

* Define a struct and Sync table struct to database

```Go
type User struct {
    Id int64
    Name string
    Salt string
    Age int
    Passwd string `xorm:"varchar(200)"`
    Created time.Time `xorm:"created"`
    Updated time.Time `xorm:"updated"`
}

err := engine.Sync(new(User))
```

* Create Engine Group

```Go
dataSourceNameSlice := []string{masterDataSourceName, slave1DataSourceName, slave2DataSourceName}
engineGroup, err := xorm.NewEngineGroup(driverName, dataSourceNameSlice)
```

```Go
masterEngine, err := xorm.NewEngine(driverName, masterDataSourceName)
slave1Engine, err := xorm.NewEngine(driverName, slave1DataSourceName)
slave2Engine, err := xorm.NewEngine(driverName, slave2DataSourceName)
engineGroup, err := xorm.NewEngineGroup(masterEngine, []*Engine{slave1Engine, slave2Engine})
```

Then all place where `engine` you can just use `engineGroup`.

* `Query` runs a SQL string, the returned results is `[]map[string][]byte`, `QueryString` returns `[]map[string]string`, `QueryInterface` returns `[]map[string]interface{}`.

```Go
results, err := engine.Query("select * from user")
results, err := engine.Where("a = 1").Query()

results, err := engine.QueryString("select * from user")
results, err := engine.Where("a = 1").QueryString()

results, err := engine.QueryInterface("select * from user")
results, err := engine.Where("a = 1").QueryInterface()
```

* `Exec` runs a SQL string, it returns `affected` and `error`

```Go
affected, err := engine.Exec("update user set age = ? where name = ?", age, name)
```

* `Insert` one or multiple records to database

```Go
affected, err := engine.Insert(&user)
// INSERT INTO struct () values ()

affected, err := engine.Insert(&user1, &user2)
// INSERT INTO struct1 () values ()
// INSERT INTO struct2 () values ()

affected, err := engine.Insert(&users)
// INSERT INTO struct () values (),(),()

affected, err := engine.Insert(&user1, &users)
// INSERT INTO struct1 () values ()
// INSERT INTO struct2 () values (),(),()

affected, err := engine.Table("user").Insert(map[string]interface{}{
    "name": "lunny",
    "age": 18,
})
// INSERT INTO user (name, age) values (?,?)

affected, err := engine.Table("user").Insert([]map[string]interface{}{
    {
        "name": "lunny",
        "age": 18,
    },
    {
        "name": "lunny2",
        "age": 19,
    },
})
// INSERT INTO user (name, age) values (?,?),(?,?)
```

* `Get` query one record from database

```Go
has, err := engine.Get(&user)
// SELECT * FROM user LIMIT 1

has, err := engine.Where("name = ?", name).Desc("id").Get(&user)
// SELECT * FROM user WHERE name = ? ORDER BY id DESC LIMIT 1

var name string
has, err := engine.Table(&user).Where("id = ?", id).Cols("name").Get(&name)
// SELECT name FROM user WHERE id = ?

var id int64
has, err := engine.Table(&user).Where("name = ?", name).Cols("id").Get(&id)
has, err := engine.SQL("select id from user").Get(&id)
// SELECT id FROM user WHERE name = ?

var id int64
var name string
has, err := engine.Table(&user).Cols("id", "name").Get(&id, &name)
// SELECT id, name FROM user LIMIT 1

var valuesMap = make(map[string]string)
has, err := engine.Table(&user).Where("id = ?", id).Get(&valuesMap)
// SELECT * FROM user WHERE id = ?

var valuesSlice = make([]interface{}, len(cols))
has, err := engine.Table(&user).Where("id = ?", id).Cols(cols...).Get(&valuesSlice)
// SELECT col1, col2, col3 FROM user WHERE id = ?
```

* `Exist` check if one record exist on table

```Go
has, err := testEngine.Exist(new(RecordExist))
// SELECT * FROM record_exist LIMIT 1

has, err = testEngine.Exist(&RecordExist{
		Name: "test1",
	})
// SELECT * FROM record_exist WHERE name = ? LIMIT 1

has, err = testEngine.Where("name = ?", "test1").Exist(&RecordExist{})
// SELECT * FROM record_exist WHERE name = ? LIMIT 1

has, err = testEngine.SQL("select * from record_exist where name = ?", "test1").Exist()
// select * from record_exist where name = ?

has, err = testEngine.Table("record_exist").Exist()
// SELECT * FROM record_exist LIMIT 1

has, err = testEngine.Table("record_exist").Where("name = ?", "test1").Exist()
// SELECT * FROM record_exist WHERE name = ? LIMIT 1
```

* `Find` query multiple records from database, also you can use join and extends

```Go
var users []User
err := engine.Where("name = ?", name).And("age > 10").Limit(10, 0).Find(&users)
// SELECT * FROM user WHERE name = ? AND age > 10 limit 10 offset 0

type Detail struct {
    Id int64
    UserId int64 `xorm:"index"`
}

type UserDetail struct {
    User `xorm:"extends"`
    Detail `xorm:"extends"`
}

var users []UserDetail
err := engine.Table("user").Select("user.*, detail.*").
    Join("INNER", "detail", "detail.user_id = user.id").
    Where("user.name = ?", name).Limit(10, 0).
    Find(&users)
// SELECT user.*, detail.* FROM user INNER JOIN detail WHERE user.name = ? limit 10 offset 0
```

* `Iterate` and `Rows` query multiple records and record by record handle, there are two methods Iterate and Rows

```Go
err := engine.Iterate(&User{Name:name}, func(idx int, bean interface{}) error {
    user := bean.(*User)
    return nil
})
// SELECT * FROM user

err := engine.BufferSize(100).Iterate(&User{Name:name}, func(idx int, bean interface{}) error {
    user := bean.(*User)
    return nil
})
// SELECT * FROM user Limit 0, 100
// SELECT * FROM user Limit 101, 100
```

You can use rows which is similiar with `sql.Rows`

```Go
rows, err := engine.Rows(&User{Name:name})
// SELECT * FROM user
defer rows.Close()
bean := new(Struct)
for rows.Next() {
    err = rows.Scan(bean)
}
```

or

```Go
rows, err := engine.Cols("name", "age").Rows(&User{Name:name})
// SELECT * FROM user
defer rows.Close()
for rows.Next() {
    var name string
    var age int
    err = rows.Scan(&name, &age)
}
```

* `Update` update one or more records, default will update non-empty and non-zero fields except when you use Cols, AllCols and so on.

```Go
affected, err := engine.ID(1).Update(&user)
// UPDATE user SET ... WHERE id = ?

affected, err := engine.Update(&user, &User{Name:name})
// UPDATE user SET ... WHERE name = ?

var ids = []int64{1, 2, 3}
affected, err := engine.In("id", ids).Update(&user)
// UPDATE user SET ... WHERE id IN (?, ?, ?)

// force update indicated columns by Cols
affected, err := engine.ID(1).Cols("age").Update(&User{Name:name, Age: 12})
// UPDATE user SET age = ?, updated=? WHERE id = ?

// force NOT update indicated columns by Omit
affected, err := engine.ID(1).Omit("name").Update(&User{Name:name, Age: 12})
// UPDATE user SET age = ?, updated=? WHERE id = ?

affected, err := engine.ID(1).AllCols().Update(&user)
// UPDATE user SET name=?,age=?,salt=?,passwd=?,updated=? WHERE id = ?
```

* `Delete` delete one or more records, Delete MUST have condition

```Go
affected, err := engine.Where(...).Delete(&user)
// DELETE FROM user WHERE ...

affected, err := engine.ID(2).Delete(&user)
// DELETE FROM user WHERE id = ?

affected, err := engine.Table("user").Where(...).Delete()
// DELETE FROM user WHERE ...
```

* `Count` count records

```Go
counts, err := engine.Count(&user)
// SELECT count(*) AS total FROM user
```

* `FindAndCount` combines function `Find` with `Count` which is usually used in query by page

```Go
var users []User
counts, err := engine.FindAndCount(&users)
```

* `Sum` sum functions

```Go
agesFloat64, err := engine.Sum(&user, "age")
// SELECT sum(age) AS total FROM user

agesInt64, err := engine.SumInt(&user, "age")
// SELECT sum(age) AS total FROM user

sumFloat64Slice, err := engine.Sums(&user, "age", "score")
// SELECT sum(age), sum(score) FROM user

sumInt64Slice, err := engine.SumsInt(&user, "age", "score")
// SELECT sum(age), sum(score) FROM user
```

* Query conditions builder

```Go
err := engine.Where(builder.NotIn("a", 1, 2).And(builder.In("b", "c", "d", "e"))).Find(&users)
// SELECT id, name ... FROM user WHERE a NOT IN (?, ?) AND b IN (?, ?, ?)
```

* Multiple operations in one go routine, no transaction here but resue session memory

```Go
session := engine.NewSession()
defer session.Close()

user1 := Userinfo{Username: "xiaoxiao", Departname: "dev", Alias: "lunny", Created: time.Now()}
if _, err := session.Insert(&user1); err != nil {
    return err
}

user2 := Userinfo{Username: "yyy"}
if _, err := session.Where("id = ?", 2).Update(&user2); err != nil {
    return err
}

if _, err := session.Exec("delete from userinfo where username = ?", user2.Username); err != nil {
    return err
}

return nil
```

* Transaction should be on one go routine. There is transaction and resue session memory

```Go
session := engine.NewSession()
defer session.Close()

// add Begin() before any action
if err := session.Begin(); err != nil {
    // if returned then will rollback automatically
    return err
}

user1 := Userinfo{Username: "xiaoxiao", Departname: "dev", Alias: "lunny", Created: time.Now()}
if _, err := session.Insert(&user1); err != nil {
    return err
}

user2 := Userinfo{Username: "yyy"}
if _, err := session.Where("id = ?", 2).Update(&user2); err != nil {
    return err
}

if _, err := session.Exec("delete from userinfo where username = ?", user2.Username); err != nil {
    return err
}

// add Commit() after all actions
return session.Commit()
```

* Or you can use `Transaction` to replace above codes.

```Go
res, err := engine.Transaction(func(session *xorm.Session) (interface{}, error) {
    user1 := Userinfo{Username: "xiaoxiao", Departname: "dev", Alias: "lunny", Created: time.Now()}
    if _, err := session.Insert(&user1); err != nil {
        return nil, err
    }

    user2 := Userinfo{Username: "yyy"}
    if _, err := session.Where("id = ?", 2).Update(&user2); err != nil {
        return nil, err
    }

    if _, err := session.Exec("delete from userinfo where username = ?", user2.Username); err != nil {
        return nil, err
    }
    return nil, nil
})
```

* Context Cache, if enabled, current query result will be cached on session and be used by next same statement on the same session.

```Go
	sess := engine.NewSession()
	defer sess.Close()

	var context = xorm.NewMemoryContextCache()

	var c2 ContextGetStruct
	has, err := sess.ID(1).ContextCache(context).Get(&c2)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, 1, c2.Id)
	assert.EqualValues(t, "1", c2.Name)
	sql, args := sess.LastSQL()
	assert.True(t, len(sql) > 0)
	assert.True(t, len(args) > 0)

	var c3 ContextGetStruct
	has, err = sess.ID(1).ContextCache(context).Get(&c3)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, 1, c3.Id)
	assert.EqualValues(t, "1", c3.Name)
	sql, args = sess.LastSQL()
	assert.True(t, len(sql) == 0)
	assert.True(t, len(args) == 0)
```

## Contributing

If you want to pull request, please see [CONTRIBUTING](CONTRIBUTING.md). And you can also go to [Xorm on discourse](https://xorm.discourse.group) to discuss.

## Credits

### Contributors

This project exists thanks to all the people who contribute. [[Contribute](CONTRIBUTING.md)].
<a href="graphs/contributors"><img src="https://opencollective.com/xorm/contributors.svg?width=890&button=false" /></a>

### Backers

Thank you to all our backers! 🙏 [[Become a backer](https://opencollective.com/xorm#backer)]

<a href="https://opencollective.com/xorm#backers" target="_blank"><img src="https://opencollective.com/xorm/backers.svg?width=890"></a>

### Sponsors

Support this project by becoming a sponsor. Your logo will show up here with a link to your website. [[Become a sponsor](https://opencollective.com/xorm#sponsor)]

## Changelog

You can find all the changelog [here](CHANGELOG.md)

## Cases

* [studygolang](http://studygolang.com/) - [github.com/studygolang/studygolang](https://github.com/studygolang/studygolang)

* [Gitea](http://gitea.io) - [github.com/go-gitea/gitea](http://github.com/go-gitea/gitea)

* [Gogs](http://try.gogits.org) - [github.com/gogits/gogs](http://github.com/gogits/gogs)

* [grafana](https://grafana.com/) - [github.com/grafana/grafana](http://github.com/grafana/grafana)

* [github.com/m3ng9i/qreader](https://github.com/m3ng9i/qreader)

* [Wego](http://github.com/go-tango/wego)

* [Docker.cn](https://docker.cn/)

* [Xorm Adapter](https://github.com/casbin/xorm-adapter) for [Casbin](https://github.com/casbin/casbin) - [github.com/casbin/xorm-adapter](https://github.com/casbin/xorm-adapter)

* [Gorevel](http://gorevel.cn/) - [github.com/goofcc/gorevel](http://github.com/goofcc/gorevel)

* [Gowalker](http://gowalker.org) - [github.com/Unknwon/gowalker](http://github.com/Unknwon/gowalker)

* [Gobuild.io](http://gobuild.io) - [github.com/shxsun/gobuild](http://github.com/shxsun/gobuild)

* [Sudo China](http://sudochina.com) - [github.com/insionng/toropress](http://github.com/insionng/toropress)

* [Godaily](http://godaily.org) - [github.com/govc/godaily](http://github.com/govc/godaily)

* [YouGam](http://www.yougam.com/)

* [GoCMS - github.com/zzboy/GoCMS](https://github.com/zzdboy/GoCMS)

* [GoBBS - gobbs.domolo.com](http://gobbs.domolo.com/)

* [go-blog](http://wangcheng.me) - [github.com/easykoo/go-blog](https://github.com/easykoo/go-blog)

## LICENSE

BSD License [http://creativecommons.org/licenses/BSD/](http://creativecommons.org/licenses/BSD/)
