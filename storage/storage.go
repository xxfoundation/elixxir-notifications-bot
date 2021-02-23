package storage

import (
	"github.com/pkg/errors"
	"gitlab.com/elixxir/crypto/hash"
)

type Storage struct {
	database
}

// Create a new Storage object wrapping a database interface
// Returns a Storage object and error
func NewStorage(username, password, dbName, address, port string) (*Storage, error) {
	db, err := newDatabase(username, password, dbName, address, port)
	storage := &Storage{db}
	return storage, err
}

func (s *Storage) AddUser(iid, transmissionRSA []byte, token string) (*User, error) {
	h, err := hash.NewCMixHash()
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to create cmix hash")
	}
	_, err = h.Write(transmissionRSA)
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to hash transmission RSA")
	}
	u := &User{
		IntermediaryId:      iid,
		TransmissionRSAHash: h.Sum(nil),
		TransmissionRSA:     transmissionRSA,
		Token:               token,
	}
	return u, s.upsertUser(u)
}

func (s *Storage) DeleteUser(transmissionRSA []byte) error {
	h, err := hash.NewCMixHash()
	if err != nil {
		return errors.WithMessage(err, "Failed to create cmix hash")
	}
	_, err = h.Write(transmissionRSA)
	if err != nil {
		return errors.WithMessage(err, "Failed to hash transmission RSA")
	}
	return s.deleteUser(h.Sum(nil))
}

func (s *Storage) DeleteUserByHash(transmissionRSAHash []byte) error {
	return s.deleteUser(transmissionRSAHash)
}
