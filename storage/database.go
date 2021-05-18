package storage

import (
	"fmt"
	jww "github.com/spf13/jwalterweatherman"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"time"
)

// interface declaration for storage methods
type database interface {
	upsertUser(user *User) error
	GetUser(iid []byte) (*User, error)
	GetUserByHash(transmissionRsaHash []byte) (*User, error)
	getUsersByOffset(offset int64) ([]*User, error)
	GetAllUsers() ([]*User, error)
	DeleteUserByHash(transmissionRsaHash []byte) error

	upsertEphemeral(ephemeral *Ephemeral) error
	GetEphemeral(ephemeralId int64) ([]*Ephemeral, error)
	GetLatestEphemeral() (*Ephemeral, error)
	DeleteOldEphemerals(currentEpoch int32) error
}

// Struct implementing the Database Interface with an underlying DB
type DatabaseImpl struct {
	db *gorm.DB // Stored database connection
}

// Struct implementing the Database Interface with an underlying Map
type MapImpl struct {
	usersById      map[string]*User
	usersByRsaHash map[string]*User
	usersByOffset  map[int64][]*User
	allUsers       []*User
	allEphemerals  map[int]*Ephemeral
	ephemeralsById map[int64][]*Ephemeral
	ephIDSeq       int
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
	// Time in which user has registered with the network (ie permissioning)
	RegistrationTimestamp time.Time `gorm:"not null"`
}

type Ephemeral struct {
	ID                  uint   `gorm:"primaryKey"`
	Offset              int64  `gorm:"not null; index"`
	TransmissionRSAHash []byte `gorm:"not null; unique; references users(transmission_rsa_hash)"`
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
			usersById:      map[string]*User{},
			usersByRsaHash: map[string]*User{},
			usersByOffset:  map[int64][]*User{},
			allUsers:       nil,
			ephemeralsById: map[int64][]*Ephemeral{},
			allEphemerals:  map[int]*Ephemeral{},
			ephIDSeq:       0,
		}

		return database(mapImpl), nil
	}

	// Initialize the database schema
	// WARNING: Order is important. Do not change without database testing
	models := []interface{}{&User{}, &Ephemeral{}}
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
