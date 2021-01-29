package sign

import (
	"filippo.io/edwards25519"
	"github.com/taurusgroup/tg-tss/pkg/frost"
)

type Signer struct {
	*frost.Party

	CommitmentD, CommitmentE *edwards25519.Point

	Rho *edwards25519.Scalar

	R *edwards25519.Point

	SigShare *edwards25519.Scalar
}

func (s *Signer) Reset() {
	zero := edwards25519.NewScalar()
	identity := edwards25519.NewIdentityPoint()

	s.CommitmentD.Set(identity)
	s.CommitmentE.Set(identity)
	s.R.Set(identity)
	s.Rho.Set(zero)
	s.SigShare.Set(zero)
}