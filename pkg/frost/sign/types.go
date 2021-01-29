package sign

import "github.com/taurusgroup/tg-tss/pkg/frost"

type (
	//KeyGenMessage struct {
	//	//Message1 *keygen.KeyGenMessage1
	//	//Message2 *keygen.KeyGenMessage2
	//}



	MessageContainer struct {
		To, From uint32
		//KeyGen *KeyGenMessage
		Sign *Message
		Signature *frost.Signature
	}
)
