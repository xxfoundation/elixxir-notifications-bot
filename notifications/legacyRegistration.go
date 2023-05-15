////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package notifications

import (
	"github.com/pkg/errors"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/crypto/hash"
	"gitlab.com/elixxir/crypto/registration"
	"gitlab.com/xx_network/crypto/signature/rsa"
	"gitlab.com/xx_network/primitives/id"
	"gitlab.com/xx_network/primitives/id/ephemeral"
	"time"
)

// RegisterForNotifications is called by the client, and adds a user registration to our database
func (nb *Impl) RegisterForNotifications(request *pb.NotificationRegisterRequest) error {
	var err error
	// Check auth & inputs
	if string(request.Token) == "" {
		return errors.New("Cannot register for notifications with empty client token")
	}

	// Verify permissioning RSA signature
	permHost, ok := nb.Comms.GetHost(&id.Permissioning)
	if !ok {
		return errors.New("Could not find permissioning host to verify client signature")
	}
	err = registration.VerifyWithTimestamp(permHost.GetPubKey(), request.RegistrationTimestamp,
		string(request.TransmissionRsa), request.TransmissionRsaSig)
	if err != nil {
		return errors.WithMessage(err, "Failed to verify perm sig with timestamp")
	}

	// Verify IID transmission RSA signature
	h, err := hash.NewCMixHash()
	if err != nil {
		return errors.WithMessage(err, "Failed to create cmix hash")
	}
	_, err = h.Write(request.IntermediaryId)
	if err != nil {
		return errors.Wrap(err, "Failed to write intermediary id to hash")
	}
	pub, err := rsa.LoadPublicKeyFromPem(request.TransmissionRsa)
	if err != nil {
		return errors.WithMessage(err, "Failed to load public key from bytes")
	}
	err = rsa.Verify(pub, hash.CMixHash, h.Sum(nil), request.IIDTransmissionRsaSig, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to verify IID signature from client")
	}

	// Add the user to storage
	_, epoch := ephemeral.HandleQuantization(time.Now())
	_, err = nb.Storage.RegisterForNotifications(request.IntermediaryId, request.TransmissionRsa, request.Token, "xxm", epoch, nb.inst.GetPartialNdf().Get().AddressSpace[0].Size)
	if err != nil {
		return errors.Wrap(err, "Failed to register user with notifications")
	}

	return nil
}

// UnregisterForNotifications is called by the client, and removes a user registration from our database
func (nb *Impl) UnregisterForNotifications(request *pb.NotificationUnregisterRequest) error {
	h, err := hash.NewCMixHash()
	if err != nil {
		return errors.WithMessage(err, "Failed to create cmix hash")
	}
	_, err = h.Write(request.IntermediaryId)
	if err != nil {
		return errors.WithMessage(err, "Failed to write intermediary id to hash")
	}

	ident, err := nb.Storage.GetIdentity(request.IntermediaryId)
	if err != nil {
		return errors.WithMessagef(err, "Failed to find user with intermediary ID %+v", request.IntermediaryId)
	}

	// Get the user by identity
	// Error if the identity has more than one registered user
	if len(ident.Users) != 1 {
		return errors.Errorf("Cannot legacy unregister an IID with more than one active user")
	}
	u := ident.Users[0]

	pub, err := rsa.LoadPublicKeyFromPem(u.TransmissionRSA)
	if err != nil {
		return errors.WithMessage(err, "Failed to load public key from database")
	}
	err = rsa.Verify(pub, hash.CMixHash, h.Sum(nil), request.IIDTransmissionRsaSig, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to verify IID signature from client")
	}
	err = nb.Storage.LegacyUnregister(request.IntermediaryId)
	if err != nil {
		return errors.Wrap(err, "Failed to unregister user with notifications")
	}
	return nil
}
