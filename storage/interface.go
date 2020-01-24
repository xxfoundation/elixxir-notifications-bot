////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles high level database interfaces and structures

package storage

import (
	"github.com/go-pg/pg"
	"github.com/go-pg/pg/orm"
	jww "github.com/spf13/jwalterweatherman"
	"sync"
)

// Struct implementing the Database Interface with an underlying DB
type DatabaseImpl struct {
	db *pg.DB // Stored database connection
}

// Struct implementing the Database Interface with an underlying Map
type MapImpl struct {
	users sync.Map
}

// Global variable for backend interaction
var NotificationsBackend *Storage

// Interface database storage operations
type Storage interface {
	// Obtain User from backend by primary key
	GetUser(userId string) (*User, error)
	// Delete User from backend by primary key
	DeleteUser(userId string) error
	// Insert or Update User into backend
	UpsertUser(user *User) error
}

// Structure representing a User in the Storage backend
type User struct {
	// Overwrite table name
	tableName struct{} `sql:"users,alias:users"`

	// User ID string
	Id string

	// User notifications token
	Token string
}

// Initialize the Storage interface with a proper backend type
func NewDatabase(username, password, database, address string) *Storage {
	// Create the database connection
	db := pg.Connect(&pg.Options{
		User:         username,
		Password:     password,
		Database:     database,
		Addr:         address,
		MaxRetries:   10,
		MinIdleConns: 1,
	})

	// Attempt to initialize the schema
	err := createSchema(db)
	var backend Storage
	if err != nil {
		// Return the map-backend interface
		// in the event there is a database error
		jww.ERROR.Printf("Unable to initialize database backend: %+v", err)
		jww.INFO.Println("Map backend initialized successfully!")
		backend = &MapImpl{}
	} else {
		// Return the database-backed interface in the event there is no error
		jww.INFO.Println("Database backend initialized successfully!")
		backend = &DatabaseImpl{
			db: db,
		}
	}
	return &backend
}

// Create the database schema
func createSchema(db *pg.DB) error {
	for _, model := range []interface{}{&User{}} {
		err := db.CreateTable(model, &orm.CreateTableOptions{
			// Ignore create table if already exists?
			IfNotExists: true,
			// Create temporary table?
			Temp: false,
			// FKConstraints causes CreateTable to create foreign key constraints
			// for has one relations. ON DELETE hook can be added using tag
			// `sql:"on_delete:RESTRICT"` on foreign key field.
			FKConstraints: true,
			// Replaces PostgreSQL data type `text` with `varchar(n)`
			// Varchar: 255
		})
		if err != nil {
			// Return error if one comes up
			return err
		}
	}
	// No error, return nil
	return nil
}
