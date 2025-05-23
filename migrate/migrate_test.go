package migrate

import (
	"fmt"
	"log"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/imkos/xorm"
)

type Person struct {
	ID   int64
	Name string
}

type Pet struct {
	ID       int64
	Name     string
	PersonID int
}

const (
	dbName = "testdb.sqlite3"
)

var migrations = []*Migration{
	{
		ID: "201608301400",
		Migrate: func(tx *xorm.Engine) error {
			return tx.Sync(&Person{})
		},
		Rollback: func(tx *xorm.Engine) error {
			return tx.DropTables(&Person{})
		},
	},
	{
		ID: "201608301430",
		Migrate: func(tx *xorm.Engine) error {
			return tx.Sync(&Pet{})
		},
		Rollback: func(tx *xorm.Engine) error {
			return tx.DropTables(&Pet{})
		},
	},
}

func TestMigration(t *testing.T) {
	_ = os.Remove(dbName)

	db, err := xorm.NewEngine("sqlite3", dbName)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}

	m := New(db, DefaultOptions, migrations)

	err = m.Migrate()
	assert.NoError(t, err)
	exists, _ := db.IsTableExist(&Person{})
	assert.True(t, exists)
	exists, _ = db.IsTableExist(&Pet{})
	assert.True(t, exists)
	assert.Equal(t, 2, tableCount(db, "migrations"))

	err = m.RollbackLast()
	assert.NoError(t, err)
	exists, _ = db.IsTableExist(&Person{})
	assert.True(t, exists)
	exists, _ = db.IsTableExist(&Pet{})
	assert.False(t, exists)
	assert.Equal(t, 1, tableCount(db, "migrations"))

	err = m.RollbackLast()
	assert.NoError(t, err)
	exists, _ = db.IsTableExist(&Person{})
	assert.False(t, exists)
	exists, _ = db.IsTableExist(&Pet{})
	assert.False(t, exists)
	assert.Equal(t, 0, tableCount(db, "migrations"))
}

func TestInitSchema(t *testing.T) {
	os.Remove(dbName)

	db, err := xorm.NewEngine("sqlite3", dbName)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}

	m := New(db, DefaultOptions, migrations)
	m.InitSchema(func(tx *xorm.Engine) error {
		if err := tx.Sync(&Person{}); err != nil {
			return err
		}
		return tx.Sync(&Pet{})
	})

	err = m.Migrate()
	assert.NoError(t, err)
	exists, _ := db.IsTableExist(&Person{})
	assert.True(t, exists)
	exists, _ = db.IsTableExist(&Pet{})
	assert.True(t, exists)
	assert.Equal(t, 2, tableCount(db, "migrations"))
}

func TestMissingID(t *testing.T) {
	os.Remove(dbName)

	db, err := xorm.NewEngine("sqlite3", dbName)
	assert.NoError(t, err)
	if db != nil {
		defer db.Close()
	}
	assert.NoError(t, db.Ping())

	migrationsMissingID := []*Migration{
		{
			Migrate: func(tx *xorm.Engine) error {
				return nil
			},
		},
	}

	m := New(db, DefaultOptions, migrationsMissingID)
	assert.Equal(t, ErrMissingID, m.Migrate())
}

func tableCount(db *xorm.Engine, tableName string) (count int) {
	_, _ = db.SQL(fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Get(&count)
	return
}
