package sign

import (
	"errors"

	"github.com/taurusgroup/frost-ed25519/pkg/eddsa"
	"github.com/taurusgroup/frost-ed25519/pkg/messages"
	"github.com/taurusgroup/frost-ed25519/pkg/state"
)

var (
	ErrValidateSigShare  = errors.New("signature share is invalid")
	ErrValidateSignature = errors.New("full signature is invalid")
)

func (round *round2) ProcessMessage(msg *messages.Message) *state.Error {
	id := msg.From()
	otherParty := round.Parties[id]
	if !eddsa.Verify(&round.C, &msg.Sign2.Zi, &otherParty.Public, &otherParty.Ri) {
		return state.NewError(id, ErrValidateSigShare)
	}
	otherParty.Zi.Set(&msg.Sign2.Zi)
	return nil
}

func (round *round2) GenerateMessages() ([]*messages.Message, *state.Error) {
	var Signature eddsa.Signature

	// S = ∑ s_i
	S := &Signature.S
	for _, otherParty := range round.Parties {
		// s += s_i
		S.Add(S, &otherParty.Zi)
	}

	Signature.R.Set(&round.R)

	// Verify the full signature here too.
	if !Signature.Verify(round.Message, &round.GroupKey) {
		return nil, state.NewError(0, ErrValidateSignature)
	}

	round.Output.Signature = &Signature
	return nil, nil
}

func (round *round2) NextRound() state.Round {
	return nil
}
