package swp

import (
	"fmt"
	"time"

	"github.com/nats-io/gnatsd/server"
	gnatsd "github.com/nats-io/gnatsd/test"
	"github.com/nats-io/nats"
	"io/ioutil"
	"net"

	"os"
)

// not meant to be run on its own, but shows an example
// of all the setup and teardown in a test.
func exampleSetup_test() {
	origdir, tempdir := MakeAndMoveToTempDir() // cd to tempdir
	p("origdir = '%s'", origdir)
	p("tempdir = '%s'", tempdir)
	defer TempDirCleanup(origdir, tempdir)

	host := "127.0.0.1"
	port := GetAvailPort()
	gnats := StartGnatsd(host, port)
	defer func() {
		p("calling gnats.Shutdown()")
		gnats.Shutdown() // when done
	}()
	gnatsdUrl := fmt.Sprintf("nats://%v:%v", host, port)

	subj := "my-topic"
	sub := StartSubscriber("B-subscriber", gnatsdUrl, subj)
	defer sub.Close()

	pub := StartPublisher("A-publisher", gnatsdUrl, subj)
	defer pub.Close()

	p("sub = %#v", sub)
	p("pub = %#v", pub)
}

func MakeAndMoveToTempDir() (origdir string, tmpdir string) {

	// make new temp dir that will have no ".goqclusterid files in it
	var err error
	origdir, err = os.Getwd()
	if err != nil {
		panic(err)
	}
	tmpdir, err = ioutil.TempDir(origdir, "temp-profiler-testdir")
	if err != nil {
		panic(err)
	}
	err = os.Chdir(tmpdir)
	if err != nil {
		panic(err)
	}

	return origdir, tmpdir
}

func TempDirCleanup(origdir string, tmpdir string) {
	// cleanup
	os.Chdir(origdir)
	err := os.RemoveAll(tmpdir)
	if err != nil {
		panic(err)
	}
	q("\n TempDirCleanup of '%s' done.\n", tmpdir)
}

// GetAvailPort asks the OS for an unused port.
// There's a race here, where the port could be grabbed by someone else
// before the caller gets to Listen on it, but in practice such races
// are rare. Uses net.Listen("tcp", ":0") to determine a free port, then
// releases it back to the OS with Listener.Close().
func GetAvailPort() int {
	l, _ := net.Listen("tcp", ":0")
	r := l.Addr()
	l.Close()
	return r.(*net.TCPAddr).Port
}

func StartGnatsd(host string, port int) *server.Server {
	//serverList := fmt.Sprintf("nats://%v:%v", host, port)

	// start yourself an embedded gnatsd server
	opts := server.Options{
		Host:  host,
		Port:  port,
		Trace: true,
		Debug: true,
	}
	gnats := gnatsd.RunServer(&opts)
	//gnats.SetLogger(&Logger{}, true, true)

	//logger := log.New(os.Stderr, "gnatsd: ", log.LUTC|log.Ldate|log.Ltime|log.Lmicroseconds|log.Llongfile)
	addr := fmt.Sprintf("%v:%v", host, port)
	if !PortIsBound(addr) {
		panic("port not bound " + addr)
	}
	return gnats
}

func PortIsBound(addr string) bool {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

type Pub struct {
	Nc           *nats.Conn
	Scrip        *nats.Subscription
	MsgArrivalCh chan *nats.Msg
	Cfg          NatsConfig
	Subject      string
}

func StartPublisher(myname string, serverList string, subject string) *Pub {
	pub := &Pub{
		MsgArrivalCh: make(chan *nats.Msg),
	}
	pub.Cfg.ServerList = serverList
	pub.Cfg.AyncErrPanics = true
	pub.Cfg.Init(myname)
	pub.Subject = subject

	nc, err := nats.Connect(serverList, pub.Cfg.opts...)
	panicOn(err)
	p("publisher client connection succeeded.")
	pub.Nc = nc

	pub.Scrip, err = nc.Subscribe(subject, func(msg *nats.Msg) {
		pub.MsgArrivalCh <- msg
	})
	panicOn(err)
	return pub
}

func (s *Pub) Close() {
	err := s.Scrip.Unsubscribe()
	panicOn(err)
	s.Nc.Close()
	p("pub unsubscribe and close done")
}

type Sub struct {
	Nc           *nats.Conn
	Scrip        *nats.Subscription
	MsgArrivalCh chan *nats.Msg
	Cfg          NatsConfig
	Subject      string
}

func StartSubscriber(myname string, serverList string, subject string) *Sub {
	sub := &Sub{
		MsgArrivalCh: make(chan *nats.Msg),
	}
	sub.Cfg.ServerList = serverList
	sub.Cfg.AyncErrPanics = true
	sub.Cfg.Init(myname)
	sub.Subject = subject

	// start client
	nc, err := nats.Connect(serverList, sub.Cfg.opts...)
	panicOn(err)
	p("subscriber client connection succeeded.")
	sub.Nc = nc

	sub.Scrip, err = nc.Subscribe(subject, func(msg *nats.Msg) {
		sub.MsgArrivalCh <- msg
	})
	panicOn(err)

	return sub
}

func (s *Sub) Close() {
	err := s.Scrip.Unsubscribe()
	panicOn(err)
	s.Nc.Close()
	p("sub unsubscribe and close done")
}

type asyncErr struct {
	conn *nats.Conn
	sub  *nats.Subscription
	err  error
}

type NatsConfig struct {
	NatsAsyncErrCh   chan asyncErr
	NatsConnClosedCh chan *nats.Conn
	NatsConnDisconCh chan *nats.Conn
	NatsConnReconCh  chan *nats.Conn

	opts  []nats.Option
	certs certConfig

	ServerList string
	SkipTLS    bool
	NatsName   string

	// helpful for test code to auto-crash on error
	AyncErrPanics bool
}

func (cfg *NatsConfig) Init(name string) {

	cfg.SkipTLS = true // for testing, not prod.
	cfg.NatsName = name

	if !cfg.SkipTLS && !cfg.certs.skipTLS {
		err := cfg.certs.certLoad()
		if err != nil {
			panic(err)
		}
	}

	o := []nats.Option{}
	o = append(o, nats.MaxReconnects(-1)) // -1 => keep trying forever
	o = append(o, nats.ReconnectWait(2*time.Second))
	o = append(o, nats.Name(cfg.NatsName))

	o = append(o, nats.ErrorHandler(func(c *nats.Conn, s *nats.Subscription, e error) {
		if cfg.AyncErrPanics {
			panic(e)
		}
		cfg.NatsAsyncErrCh <- asyncErr{conn: c, sub: s, err: e}
	}))
	o = append(o, nats.DisconnectHandler(func(conn *nats.Conn) {
		cfg.NatsConnDisconCh <- conn
	}))
	o = append(o, nats.ReconnectHandler(func(conn *nats.Conn) {
		cfg.NatsConnReconCh <- conn
	}))
	o = append(o, nats.ClosedHandler(func(conn *nats.Conn) {
		cfg.NatsConnClosedCh <- conn
	}))

	if !cfg.SkipTLS && !cfg.certs.skipTLS {
		o = append(o, nats.Secure(&cfg.certs.tlsConfig))
		o = append(o, cfg.certs.rootCA)
	}

	cfg.opts = o
}
