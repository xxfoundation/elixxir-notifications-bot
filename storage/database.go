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
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"time"
)

const postgresConnectString = "host=%s port=%s user=%s dbname=%s sslmode=disable"
const sqliteDatabasePath = "file:%s?mode=memory&cache=shared"

// interface declaration for storage methods
type database interface {
	UpsertState(state *State) error
	GetStateValue(key string) (string, error)

	insertUser(user *User) error
	GetUser(transmissionRsaHash []byte) (*User, error)
	deleteUser(transmissionRsaHash []byte) error
	GetAllUsers() ([]*User, error)

	registerTrackedIdentity(user User, identity Identity) error
	registerTrackedIdentities(user User, ids []Identity) error

	GetIdentity(iid []byte) (*Identity, error)
	insertIdentity(identity *Identity) error
	getIdentitiesByOffset(offset int64) ([]*Identity, error)
	GetOrphanedIdentities() ([]*Identity, error)

	insertEphemeral(ephemeral *Ephemeral) error
	GetEphemeral(ephemeralId int64) ([]*Ephemeral, error)
	GetLatestEphemeral() (*Ephemeral, error)
	DeleteOldEphemerals(currentEpoch int32) error
	GetToNotify(ephemeralIds []int64) ([]GTNResult, error)

	insertToken(token Token) error
	DeleteToken(token string) error

	unregisterIdentities(u *User, iids []Identity) error
	unregisterTokens(u *User, tokens []Token) error
	registerForNotifications(u *User, identity Identity, token Token) error
	LegacyUnregister(iid []byte) error
}

// DatabaseImpl is a struct which implements database on an underlying gorm.DB
type DatabaseImpl struct {
	db *gorm.DB // Stored database connection
}

// State table
type State struct {
	Key   string `gorm:"primary_key"`
	Value string `gorm:"NOT NULL"`
}

type UserV1 struct {
	TransmissionRSAHash []byte      `gorm:"primaryKey"`
	IntermediaryId      []byte      `gorm:"not null; index"`
	OffsetNum           int64       `gorm:"not null; index"`
	TransmissionRSA     []byte      `gorm:"not null"`
	Signature           []byte      `gorm:"not null"`
	Token               string      `gorm:"not null"`
	Ephemerals          []Ephemeral `gorm:"foreignKey:transmission_rsa_hash;references:transmission_rsa_hash;constraint:OnDelete:CASCADE;"`
}

type Token struct {
	Token               string `gorm:"primaryKey"`
	App                 string
	TransmissionRSAHash []byte `gorm:"not null;references users(transmission_rsa_hash)"`
}

type User struct {
	TransmissionRSAHash []byte     `gorm:"primaryKey"`
	TransmissionRSA     []byte     `gorm:"not null"`
	Tokens              []Token    `gorm:"foreignKey:TransmissionRSAHash;constraint:OnDelete:CASCADE;"`
	Identities          []Identity `gorm:"many2many:user_identities;"`
}

// CREATES JOIN TABLE user_identities
// Table "public.user_identities"
// Column           | Type  | Collation | Nullable | Default
// ----------------------------+-------+-----------+----------+---------
// user_transmission_rsa_hash | bytea |           | not null |
// identity_intermediary_id   | bytea |           | not null |
// Indexes:
// "user_identities_pkey" PRIMARY KEY, btree (user_transmission_rsa_hash, identity_intermediary_id)
// Foreign-key constraints:
// "fk_user_identities_identity" FOREIGN KEY (identity_intermediary_id) REFERENCES identities(intermediary_id)
// "fk_user_identities_user" FOREIGN KEY (user_transmission_rsa_hash) REFERENCES users(transmission_rsa_hash)

type Identity struct {
	IntermediaryId []byte      `gorm:"primaryKey"`
	OffsetNum      int64       `gorm:"not null; index"`
	Users          []User      `gorm:"many2many:user_identities;"`
	Ephemerals     []Ephemeral `gorm:"foreignKey:intermediary_id;references:intermediary_id;constraint:OnDelete:CASCADE;"`
}

type Ephemeral struct {
	ID             uint   `gorm:"primaryKey"`
	IntermediaryId []byte `gorm:"not null;references identities(intermediary_id)"`
	EphemeralId    int64  `gorm:"not null; index"`
	Epoch          int32  `gorm:"not null; index"`
}

// Initialize the database interface with database backend
// Returns a database interface, close function, and error
func newDatabase(username, password, dbName, address,
	port string) (database, error) {
	var err error
	var db *gorm.DB
	var dialector gorm.Dialector
	// Connect to the database if the correct information is provided
	usePostgres := address != "" && port != ""
	if usePostgres {
		// Create the database connection
		connectString := fmt.Sprintf(
			postgresConnectString,
			address, port, username, dbName)
		// Handle empty database password
		if len(password) > 0 {
			connectString += fmt.Sprintf(" password=%s", password)
		}
		dialector = postgres.Open(connectString)
	} else {
		jww.WARN.Printf("Database backend connection information not provided")
		temporaryDbPath := fmt.Sprintf(sqliteDatabasePath, dbName)
		dialector = sqlite.Open(temporaryDbPath)
	}

	// Create the database connection
	db, err = gorm.Open(dialector, &gorm.Config{
		Logger: logger.New(jww.TRACE, logger.Config{LogLevel: logger.Info}),
	})
	if err != nil {
		return nil, errors.Errorf("Unable to initialize in-memory sqlite database backend: %+v", err)
	}

	if !usePostgres {
		// Enable foreign keys because they are disabled in SQLite by default
		if err = db.Exec("PRAGMA foreign_keys = ON", nil).Error; err != nil {
			return nil, err
		}

		// Enable Write Ahead Logging to enable multiple DB connections
		if err = db.Exec("PRAGMA journal_mode = WAL;", nil).Error; err != nil {
			return nil, err
		}
	}

	// Get and configure the internal database ConnPool
	sqlDb, err := db.DB()
	if err != nil {
		return nil, errors.Errorf("Unable to configure database connection pool: %+v", err)
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
	models := []interface{}{&Token{}, &User{}, &Identity{}, &Ephemeral{}, &State{}}
	for _, model := range models {
		err = db.AutoMigrate(model)
		if err != nil {
			return nil, err
		}
	}

	// Build the interface
	di := &DatabaseImpl{
		db: db,
	}

	jww.INFO.Println("Database backend initialized successfully!")
	return database(di), nil
}
