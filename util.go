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

	subC := NewNatsClientConfig(host, port, "B-subscriber", "toB", true, true)
	sub := NewNatsClient(subC)
	err := sub.Start()
	panicOn(err)
	defer sub.Close()

	pubC := NewNatsClientConfig(host, port, "A-publisher", "toA", true, true)
	pub := NewNatsClient(pubC)
	err = pub.Start()
	panicOn(err)
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

type NatsClient struct {
	Nc           *nats.Conn
	Scrip        *nats.Subscription
	MsgArrivalCh chan *nats.Msg
	Cfg          *NatsClientConfig
	Subject      string
}

func NewNatsClient(cfg *NatsClientConfig) *NatsClient {
	c := &NatsClient{
		MsgArrivalCh: make(chan *nats.Msg),
		Cfg:          cfg,
		Subject:      cfg.Subject,
	}
	return c
}

func (s *NatsClient) Start() error {
	//	s.Cfg.AsyncErrPanics = true
	s.Cfg.Init()

	nc, err := nats.Connect(s.Cfg.ServerList, s.Cfg.Opts...)
	panicOn(err)
	p("client connection succeeded.")
	s.Nc = nc

	return nil
}

func (s *NatsClient) MakeSub(subject string, hand nats.MsgHandler) error {
	var err error
	s.Scrip, err = s.Nc.Subscribe(s.Subject, hand)
	panicOn(err)
	return err
}

func (s *NatsClient) Close() {
	if s.Scrip != nil {
		err := s.Scrip.Unsubscribe()
		panicOn(err)
	}
	if s.Nc != nil {
		s.Nc.Close()
	}
	p("NatsClient unsubscribe and close done")
}

type asyncErr struct {
	conn *nats.Conn
	sub  *nats.Subscription
	err  error
}

func NewNatsClientConfig(
	host string,
	port int,
	myname string,
	subject string,
	skipTLS bool,
	asyncErrCrash bool) *NatsClientConfig {

	cfg := &NatsClientConfig{
		Host:           host,
		Port:           port,
		NatsNodeName:   myname,
		Subject:        subject,
		SkipTLS:        skipTLS,
		AsyncErrPanics: asyncErrCrash,
		ServerList:     fmt.Sprintf("nats://%v:%v", host, port),
	}
	return cfg
}

type NatsClientConfig struct {
	// ====================
	// user supplied
	// ====================
	Host string
	Port int

	NatsNodeName string
	Subject      string

	SkipTLS bool

	// helpful for test code to auto-crash on error
	AsyncErrPanics bool

	// ====================
	// Init() fills in:
	// ====================
	ServerList string

	NatsAsyncErrCh   chan asyncErr
	NatsConnClosedCh chan *nats.Conn
	NatsConnDisconCh chan *nats.Conn
	NatsConnReconCh  chan *nats.Conn

	Opts  []nats.Option
	Certs certConfig
}

func (cfg *NatsClientConfig) Init() {

	if !cfg.SkipTLS && !cfg.Certs.skipTLS {
		err := cfg.Certs.certLoad()
		if err != nil {
			panic(err)
		}
	}

	o := []nats.Option{}
	o = append(o, nats.MaxReconnects(-1)) // -1 => keep trying forever
	o = append(o, nats.ReconnectWait(2*time.Second))
	o = append(o, nats.Name(cfg.NatsNodeName))

	o = append(o, nats.ErrorHandler(func(c *nats.Conn, s *nats.Subscription, e error) {
		if cfg.AsyncErrPanics {
			fmt.Printf("\n  got an asyn err, here is the"+
				" status of nats queues: '%#v'\n",
				ReportOnSubscription(s))
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

	if !cfg.SkipTLS && !cfg.Certs.skipTLS {
		o = append(o, nats.Secure(&cfg.Certs.tlsConfig))
		o = append(o, cfg.Certs.rootCA)
	}

	cfg.Opts = o
}
