// Copyright 2017 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tests

import (
	"os"
	"testing"
	"time"

	"github.com/imkos/xorm/caches"
	"github.com/imkos/xorm/schemas"

	"github.com/stretchr/testify/assert"
)

func TestDelete(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	type UserinfoDelete struct {
		Uid   int64 `xorm:"id pk not null autoincr"`
		IsMan bool
	}

	assert.NoError(t, testEngine.Sync(new(UserinfoDelete)))

	session := testEngine.NewSession()
	defer session.Close()

	var err error
	if testEngine.Dialect().URI().DBType == schemas.MSSQL {
		err = session.Begin()
		assert.NoError(t, err)
		_, err = session.Exec("SET IDENTITY_INSERT userinfo_delete ON")
		assert.NoError(t, err)
	}

	user := UserinfoDelete{Uid: 1}
	cnt, err := session.Insert(&user)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	if testEngine.Dialect().URI().DBType == schemas.MSSQL {
		err = session.Commit()
		assert.NoError(t, err)
	}

	cnt, err = testEngine.Delete(&UserinfoDelete{Uid: user.Uid})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	user.Uid = 0
	user.IsMan = true
	has, err := testEngine.ID(1).Get(&user)
	assert.NoError(t, err)
	assert.False(t, has)

	cnt, err = testEngine.Insert(&user)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	cnt, err = testEngine.Where("`id`=?", user.Uid).Delete(&UserinfoDelete{})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	user.Uid = 0
	user.IsMan = true
	has, err = testEngine.ID(2).Get(&user)
	assert.NoError(t, err)
	assert.False(t, has)
}

func TestDeleteLimit(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	if testEngine.Dialect().URI().DBType == schemas.MSSQL || os.Getenv("IGNORE_TEST_DELETE_LIMIT") == "true" {
		t.Skip()
		return
	}

	type UserinfoDeleteLimit struct {
		Uid   int64 `xorm:"id pk not null autoincr"`
		IsMan bool
	}

	assert.NoError(t, testEngine.Sync2(new(UserinfoDeleteLimit)))

	session := testEngine.NewSession()
	defer session.Close()

	var err error
	if testEngine.Dialect().URI().DBType == schemas.MSSQL {
		err = session.Begin()
		assert.NoError(t, err)
		_, err = session.Exec("SET IDENTITY_INSERT userinfo_delete_limit ON")
		assert.NoError(t, err)
	}

	user := UserinfoDeleteLimit{Uid: 1, IsMan: true}
	cnt, err := session.Insert(&user)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	user2 := UserinfoDeleteLimit{Uid: 2}
	cnt, err = session.Insert(&user2)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	if testEngine.Dialect().URI().DBType == schemas.MSSQL {
		err = session.Commit()
		assert.NoError(t, err)
	}

	cnt, err = testEngine.Limit(1, 1).Delete(&UserinfoDeleteLimit{})
	assert.Error(t, err)
	assert.EqualValues(t, 0, cnt)

	cnt, err = testEngine.Limit(1).Desc("id").Delete(&UserinfoDeleteLimit{})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	var users []UserinfoDeleteLimit
	err = testEngine.Find(&users)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, len(users))
	assert.EqualValues(t, 1, users[0].Uid)
	assert.EqualValues(t, true, users[0].IsMan)
}

func TestDeleted(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	type Deleted struct {
		Id        int64 `xorm:"pk"`
		Name      string
		DeletedAt time.Time `xorm:"deleted"`
	}

	err := testEngine.DropTables(&Deleted{})
	assert.NoError(t, err)

	err = testEngine.CreateTables(&Deleted{})
	assert.NoError(t, err)

	_, err = testEngine.InsertOne(&Deleted{Id: 1, Name: "11111"})
	assert.NoError(t, err)

	_, err = testEngine.InsertOne(&Deleted{Id: 2, Name: "22222"})
	assert.NoError(t, err)

	_, err = testEngine.InsertOne(&Deleted{Id: 3, Name: "33333"})
	assert.NoError(t, err)

	// Test normal Find()
	var records1 []Deleted
	err = testEngine.Where("`"+testEngine.GetColumnMapper().Obj2Table("Id")+"` > 0").Find(&records1, &Deleted{})
	assert.NoError(t, err)
	assert.EqualValues(t, 3, len(records1))

	// Test normal Get()
	record1 := &Deleted{}
	has, err := testEngine.ID(1).Get(record1)
	assert.NoError(t, err)
	assert.True(t, has)

	// Test Delete() with deleted
	affected, err := testEngine.ID(1).Delete(&Deleted{})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, affected)

	has, err = testEngine.ID(1).Get(&Deleted{})
	assert.NoError(t, err)
	assert.False(t, has)

	var records2 []Deleted
	err = testEngine.Where("`" + testEngine.GetColumnMapper().Obj2Table("Id") + "` > 0").Find(&records2)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, len(records2))

	// Test no rows affected after Delete() again.
	affected, err = testEngine.ID(1).Delete(&Deleted{})
	assert.NoError(t, err)
	assert.EqualValues(t, 0, affected)

	// Deleted.DeletedAt must not be updated.
	affected, err = testEngine.ID(2).Update(&Deleted{Name: "2", DeletedAt: time.Now()})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, affected)

	record2 := &Deleted{}
	has, err = testEngine.ID(2).Get(record2)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.True(t, record2.DeletedAt.IsZero())

	// Test find all records whatever `deleted`.
	var unscopedRecords1 []Deleted
	err = testEngine.Unscoped().Where("`"+testEngine.GetColumnMapper().Obj2Table("Id")+"` > 0").Find(&unscopedRecords1, &Deleted{})
	assert.NoError(t, err)
	assert.EqualValues(t, 3, len(unscopedRecords1))

	// Delete() must really delete a record with Unscoped()
	affected, err = testEngine.Unscoped().ID(1).Delete(&Deleted{})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, affected)

	var unscopedRecords2 []Deleted
	err = testEngine.Unscoped().Where("`"+testEngine.GetColumnMapper().Obj2Table("Id")+"` > 0").Find(&unscopedRecords2, &Deleted{})
	assert.NoError(t, err)
	assert.EqualValues(t, 2, len(unscopedRecords2))

	var records3 []Deleted
	err = testEngine.Where("`"+testEngine.GetColumnMapper().Obj2Table("Id")+"` > 0").And("`"+testEngine.GetColumnMapper().Obj2Table("Id")+"`> 1").
		Or("`"+testEngine.GetColumnMapper().Obj2Table("Id")+"` = ?", 3).Find(&records3)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, len(records3))
}

func TestCacheDelete(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	oldCacher := testEngine.GetDefaultCacher()
	cacher := caches.NewLRUCacher(caches.NewMemoryStore(), 1000)
	testEngine.SetDefaultCacher(cacher)

	type CacheDeleteStruct struct {
		Id int64
	}

	err := testEngine.CreateTables(&CacheDeleteStruct{})
	assert.NoError(t, err)

	_, err = testEngine.Insert(&CacheDeleteStruct{})
	assert.NoError(t, err)

	aff, err := testEngine.Delete(&CacheDeleteStruct{
		Id: 1,
	})
	assert.NoError(t, err)
	assert.EqualValues(t, aff, 1)

	aff, err = testEngine.Unscoped().Delete(&CacheDeleteStruct{
		Id: 1,
	})
	assert.NoError(t, err)
	assert.EqualValues(t, aff, 0)

	testEngine.SetDefaultCacher(oldCacher)
}

func TestUnscopeDelete(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	type UnscopeDeleteStruct struct {
		Id        int64
		Name      string
		DeletedAt time.Time `xorm:"deleted"`
	}

	assertSync(t, new(UnscopeDeleteStruct))

	cnt, err := testEngine.Insert(&UnscopeDeleteStruct{
		Name: "test",
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	nowUnix := time.Now().Unix()
	var s UnscopeDeleteStruct
	cnt, err = testEngine.ID(1).Delete(&s)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)
	assert.LessOrEqual(t, int(s.DeletedAt.Unix()-nowUnix), 1)

	var s1 UnscopeDeleteStruct
	has, err := testEngine.ID(1).Get(&s1)
	assert.NoError(t, err)
	assert.False(t, has)

	var s2 UnscopeDeleteStruct
	has, err = testEngine.ID(1).Unscoped().Get(&s2)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, "test", s2.Name)
	assert.LessOrEqual(t, int(s2.DeletedAt.Unix()-nowUnix), 1)

	cnt, err = testEngine.ID(1).Unscoped().Delete(new(UnscopeDeleteStruct))
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	var s3 UnscopeDeleteStruct
	has, err = testEngine.ID(1).Get(&s3)
	assert.NoError(t, err)
	assert.False(t, has)

	var s4 UnscopeDeleteStruct
	has, err = testEngine.ID(1).Unscoped().Get(&s4)
	assert.NoError(t, err)
	assert.False(t, has)
}

func TestDelete2(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	type UserinfoDelete2 struct {
		Uid   int64 `xorm:"id pk not null autoincr"`
		IsMan bool
	}

	assert.NoError(t, testEngine.Sync(new(UserinfoDelete2)))

	user := UserinfoDelete2{}
	cnt, err := testEngine.Insert(&user)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	cnt, err = testEngine.Table("userinfo_delete2").In("id", []int{1}).Delete()
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	user2 := UserinfoDelete2{}
	has, err := testEngine.ID(1).Get(&user2)
	assert.NoError(t, err)
	assert.False(t, has)
}

func TestTruncate(t *testing.T) {
	assert.NoError(t, PrepareEngine())

	type TruncateUser struct {
		Uid int64 `xorm:"id pk not null autoincr"`
	}

	assert.NoError(t, testEngine.Sync(new(TruncateUser)))

	cnt, err := testEngine.Insert(&TruncateUser{})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	_, err = testEngine.Delete(&TruncateUser{})
	assert.Error(t, err)

	_, err = testEngine.Truncate(&TruncateUser{})
	assert.NoError(t, err)

	user2 := TruncateUser{}
	has, err := testEngine.ID(1).Get(&user2)
	assert.NoError(t, err)
	assert.False(t, has)
}
