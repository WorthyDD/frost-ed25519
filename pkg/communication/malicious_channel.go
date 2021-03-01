package communication

import (
	"encoding/hex"
	"log"
	"math/rand"
	"sync"

	"filippo.io/edwards25519"
	"github.com/taurusgroup/frost-ed25519/pkg/messages"
)

type MonkeyChannel struct {
	channels map[uint32]chan []byte
	incoming chan *messages.Message
	receiver uint32
	wg       *sync.WaitGroup
	done     chan struct{}

	chosenType messages.MessageType
}

func (c *MonkeyChannel) manipulate(msg *messages.Message) {
	if msg.From == 42 {
		randPtIdx := rand.Intn(len(order8))
		randPt := order8[randPtIdx]
		if msg.Sign1 != nil && c.chosenType == messages.MessageTypeSign1 {
			m := msg.Sign1
			m.Di.Add(&m.Di, randPt)
		}
		if msg.Sign1 != nil && c.chosenType == messages.MessageTypeSign2 {
			m := msg.Sign1
			m.Ei.Add(&m.Ei, randPt)
		}
		if msg.KeyGen1 != nil && c.chosenType == messages.MessageTypeKeyGen1 {
			m := msg.KeyGen1
			q := m.Commitments.AddConstant(randPt)
			m.Commitments = q
		}
	}
}

func (c *MonkeyChannel) Send(msg *messages.Message) error {
	c.manipulate(msg)
	b, err := msg.MarshalBinary()
	if err != nil {
		return err
	}
	if msg.To == 0 {
		for id, ch := range c.channels {
			if id != c.receiver {
				ch <- b
			}
		}
	} else if msg.To != c.receiver {
		c.channels[msg.To] <- b
	}
	return nil
}

func (c *MonkeyChannel) Incoming() <-chan *messages.Message {
	return c.incoming
}

func (c *MonkeyChannel) Done() {
	c.wg.Done()
	close(c.incoming)
	c.incoming = nil
}

func (c *MonkeyChannel) handleByteChan() {
	for {
		select {
		case <-c.done:
			return
		case data := <-c.channels[c.receiver]:
			if data == nil {
				continue
			}
			var msg messages.Message
			err := msg.UnmarshalBinary(data)
			if err != nil {
				log.Print(err)
				continue
			}
			if c.incoming != nil {
				c.incoming <- &msg
			}
		}
	}
}

func NewMonkeyChannelCommunicatorForAll(partyIDs []uint32, chosentype messages.MessageType) map[uint32]*MonkeyChannel {
	var wg sync.WaitGroup

	n := len(partyIDs)
	wg.Add(n)
	done := make(chan struct{})

	byteChannels := make(map[uint32]chan []byte, n)
	for _, id := range partyIDs {
		byteChannels[id] = make(chan []byte, n)
	}
	go waitForFinish(&wg, done, byteChannels)

	cs := make(map[uint32]*MonkeyChannel, n)
	for _, id := range partyIDs {
		incoming := make(chan *messages.Message, n)
		c := &MonkeyChannel{
			channels:   byteChannels,
			incoming:   incoming,
			receiver:   id,
			wg:         &wg,
			done:       done,
			chosenType: chosentype,
		}
		go c.handleByteChan()
		cs[id] = c
	}
	return cs
}

var order8 []*edwards25519.Point

func init() {
	order8Hex := []string{
		"ECFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF7F",
		"0000000000000000000000000000000000000000000000000000000000000080",
		"0000000000000000000000000000000000000000000000000000000000000000",
		"C7176A703D4DD84FBA3C0B760D10670F2A2053FA2C39CCC64EC7FD7792AC037A",
		"C7176A703D4DD84FBA3C0B760D10670F2A2053FA2C39CCC64EC7FD7792AC03FA",
		"26E8958FC2B227B045C3F489F2EF98F0D5DFAC05D3C63339B13802886D53FC05",
		"26E8958FC2B227B045C3F489F2EF98F0D5DFAC05D3C63339B13802886D53FC85",
	}
	for _, h := range order8Hex {
		b, _ := hex.DecodeString(h)
		p, _ := new(edwards25519.Point).SetBytes(b)
		order8 = append(order8, p)
	}
}
