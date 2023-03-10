////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	"fmt"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"sync"
	"time"
)

// interface declaration for storage methods
type database interface {
	UpsertState(state *State) error
	GetStateValue(key string) (string, error)
	upsertUser(user *User) error
	GetUser(iid []byte) (*User, error)
	GetUserByHash(transmissionRsaHash []byte) (*User, error)
	getUsersByOffset(offset int64) ([]*User, error)
	GetAllUsers() ([]*User, error)
	GetOrphanedUsers() ([]*User, error)
	DeleteUserByHash(transmissionRsaHash []byte) error

	insertEphemeral(ephemeral *Ephemeral) error
	GetEphemeral(ephemeralId int64) ([]*Ephemeral, error)
	GetLatestEphemeral() (*Ephemeral, error)
	DeleteOldEphemerals(currentEpoch int32) error
	GetToNotify(ephemeralIds []int64) ([]GTNResult, error)
}

// Struct implementing the Database Interface with an underlying DB
type DatabaseImpl struct {
	db *gorm.DB // Stored database connection
}

// Struct implementing the Database Interface with an underlying Map
type MapImpl struct {
	mut              sync.Mutex
	states           map[string]string
	usersById        map[string]*User
	usersByRsaHash   map[string]*User
	usersByOffset    map[int64][]*User
	allUsers         []*User
	allEphemerals    map[int]*Ephemeral
	ephemeralsById   map[int64][]*Ephemeral
	ephemeralsByUser map[string]map[int64]*Ephemeral
	ephIDSeq         int
}

type State struct {
	Key   string `gorm:"primary_key"`
	Value string `gorm:"NOT NULL"`
}

// Structure representing a User in the Storage backend
type User struct {
	TransmissionRSAHash []byte      `gorm:"primaryKey"`
	IntermediaryId      []byte      `gorm:"not null; index"`
	OffsetNum           int64       `gorm:"not null; index"`
	TransmissionRSA     []byte      `gorm:"not null"`
	Signature           []byte      `gorm:"not null"`
	Token               string      `gorm:"not null"`
	Ephemerals          []Ephemeral `gorm:"foreignKey:transmission_rsa_hash;references:transmission_rsa_hash;constraint:OnDelete:CASCADE;"`
}

type Ephemeral struct {
	ID                  uint   `gorm:"primaryKey"`
	Offset              int64  `gorm:"not null; index"`
	TransmissionRSAHash []byte `gorm:"not null; references users(transmission_rsa_hash)"`
	EphemeralId         int64  `gorm:"not null; index"`
	Epoch               int32  `gorm:"not null; index"`
}

// Initialize the database interface with database backend
// Returns a database interface, close function, and error
func newDatabase(username, password, dbName, address,
	port string) (database, error) {

	var err error
	var db *gorm.DB
	// Connect to the database if the correct information is provided
	if address != "" && port != "" {
		// Create the database connection
		connectString := fmt.Sprintf(
			"host=%s port=%s user=%s dbname=%s sslmode=disable",
			address, port, username, dbName)
		// Handle empty database password
		if len(password) > 0 {
			connectString += fmt.Sprintf(" password=%s", password)
		}
		db, err = gorm.Open(postgres.Open(connectString), &gorm.Config{
			Logger: logger.New(jww.TRACE, logger.Config{LogLevel: logger.Info}),
		})
	}

	// Return the map-backend interface
	// in the event there is a database error or information is not provided
	if (address == "" || port == "") || err != nil {

		if err != nil {
			jww.WARN.Printf("Unable to initialize database backend: %+v", err)
		} else {
			jww.WARN.Printf("Database backend connection information not provided")
		}

		defer jww.INFO.Println("Map backend initialized successfully!")

		mapImpl := &MapImpl{
			usersById:        map[string]*User{},
			usersByRsaHash:   map[string]*User{},
			usersByOffset:    map[int64][]*User{},
			allUsers:         nil,
			ephemeralsById:   map[int64][]*Ephemeral{},
			allEphemerals:    map[int]*Ephemeral{},
			ephemeralsByUser: map[string]map[int64]*Ephemeral{},
			states:           map[string]string{},
			ephIDSeq:         0,
		}

		return database(mapImpl), nil
	}

	// Get and configure the internal database ConnPool
	sqlDb, err := db.DB()
	if err != nil {
		return database(&DatabaseImpl{}), errors.Errorf("Unable to configure database connection pool: %+v", err)
	}
	// SetMaxIdleConns sets the maximum number of connections in the idle connection pool.
	sqlDb.SetMaxIdleConns(10)
	// SetMaxOpenConns sets the maximum number of open connections to the Database.
	sqlDb.SetMaxOpenConns(50)
	// SetConnMaxLifetime sets the maximum amount of time a connection may be idle.
	sqlDb.SetConnMaxIdleTime(10 * time.Minute)
	// SetConnMaxLifetime sets the maximum amount of time a connection may be reused.
	sqlDb.SetConnMaxLifetime(12 * time.Hour)

	// Initialize the database schema
	// WARNING: Order is important. Do not change without database testing
	models := []interface{}{&User{}, &Ephemeral{}, &State{}}
	for _, model := range models {
		err = db.AutoMigrate(model)
		if err != nil {
			return database(&DatabaseImpl{}), err
		}
	}

	// Build the interface
	di := &DatabaseImpl{
		db: db,
	}

	jww.INFO.Println("Database backend initialized successfully!")
	return database(di), nil
}
