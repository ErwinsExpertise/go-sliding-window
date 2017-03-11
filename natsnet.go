package swp

import (
	"sync"

	"github.com/glycerine/idem"
	"github.com/glycerine/nats"
)

// NatsNet connects to nats using the Network interface.
type NatsNet struct {
	Cli  *NatsClient
	mut  sync.Mutex
	Halt *idem.Halter
}

// NewNatsNet makes a new NataNet based on an actual nats client.
func NewNatsNet(cli *NatsClient) *NatsNet {
	net := &NatsNet{
		Cli:  cli,
		Halt: idem.NewHalter(),
	}
	return net
}

// BufferCaps returns the byte and message limits
// currently in effect, so that flow control
// can be used to avoid sender overrunning them.
func (n *NatsNet) BufferCaps() (bytecap int64, msgcap int64) {
	n.mut.Lock()
	defer n.mut.Unlock()
	return GetSubscripCap(n.Cli.Scrip)
}

// Listen starts receiving packets addressed to inbox on the returned channel.
func (n *NatsNet) Listen(inbox string) (chan *Packet, error) {
	mr := make(chan *Packet)

	// do actual subscription
	err := n.Cli.MakeSub(inbox, func(msg *nats.Msg) {
		var pack Packet
		_, err := pack.UnmarshalMsg(msg.Data)
		panicOn(err)
		select {
		case mr <- &pack:
		case <-n.Halt.ReqStop.Chan:
		}
	})
	//p("subscription by %v on subject %v succeeded", n.Cli.Cfg.NatsNodeName, inbox)
	return mr, err
}

// Send blocks until Send has started (but not until acked).
func (n *NatsNet) Send(pack *Packet, why string) error {
	//p("in NatsNet.Send(pack=%#v) why: '%s'", *pack, why)
	bts, err := pack.MarshalMsg(nil)
	if err != nil {
		return err
	}
	return n.Cli.Nc.Publish(pack.Dest, bts)
}

func (n *NatsNet) Stop() {
	n.Halt.RequestStop()
	n.Halt.Done.Close()
}
