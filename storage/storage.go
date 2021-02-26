package storage

import (
	"github.com/pkg/errors"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"time"
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

func (s *Storage) AddUser(iid, transmissionRSA, signature []byte, token string) (*User, error) {
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
		Signature:           signature,
		Offset:              ephemeral.GetOffset(iid),
		Token:               token,
	}
	return u, s.upsertUser(u)
}

func (s *Storage) AddLatestEphemeral(u *User) error {
	eid, _, end, err := ephemeral.GetIdFromIntermediary(u.IntermediaryId, 16, time.Now().UnixNano())
	if err != nil {
		return errors.WithMessage(err, "Failed to get ephemeral id for user")
	}
	return s.upsertEphemeral(&Ephemeral{
		TransmissionRSAHash: u.TransmissionRSAHash,
		EphemeralId:         eid[:],
		ValidUntil:          end,
		Offset:              u.Offset,
	})
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

func (s *Storage) UpdateEphemeralsForOffset(offset int64, end time.Time) error {
	users, err := s.getUsersByOffset(offset)
	if err != nil {
		return errors.WithMessage(err, "Failed to get users for given offset")
	}
	for _, u := range users {
		eid, _, end, err := ephemeral.GetIdFromIntermediary(u.IntermediaryId, 16, end.UnixNano()+1)
		if err != nil {
			return errors.WithMessage(err, "Failed to get eid for user")
		}
		err = s.upsertEphemeral(&Ephemeral{
			TransmissionRSAHash: u.TransmissionRSAHash,
			EphemeralId:         eid[:],
			ValidUntil:          end,
			Offset:              offset,
		})
		if err != nil {
			return errors.WithMessage(err, "Failed to upsert ephemeral ID for user")
		}
	}
	return nil
}
