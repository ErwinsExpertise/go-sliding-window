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

func setupTest() {
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
	subscriber := StartSubscriber(gnatsdUrl, subj)
	//	defer subscriber.Close()

	publisher := StartPublisher(gnatsdUrl, subj)
	//	defer publisher.Close()

	p("sub = %#v", subscriber)
	p("pub = %#v", publisher)
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
	Sub          *nats.Subscription
	MsgArrivalCh chan *nats.Msg
	Cfg          NatsConfig
	Subject      string
}

func StartPublisher(serverList string, subject string) *Pub {
	// start client
	asyncHandler := nats.ErrorHandler(func(c *nats.Conn, s *nats.Subscription, e error) {
		fmt.Printf("\n *** async error handler sees error: '%s'\n", e)
		panic(e)
	})
	p("about to connect with client")
	nc, err := nats.Connect(serverList, asyncHandler)
	panicOn(err)
	p("nc client connection succeeded.")

	//	err = fm.Subscribe(subject, ch, &fm.archiveSub, true)
	//	panicOn(err)

	return &Pub{Nc: nc}
}

type Sub struct {
	Nc           *nats.Conn
	Sub          *nats.Subscription
	MsgArrivalCh chan *nats.Msg
	Cfg          NatsConfig
	Subject      string
}

func StartSubscriber(serverList string, subject string) *Sub {
	// start client
	asyncHandler := nats.ErrorHandler(func(c *nats.Conn, s *nats.Subscription, e error) {
		fmt.Printf("\n *** async error handler sees error: '%s'\n", e)
		panic(e)
	})
	p("about to connect with client")
	nc, err := nats.Connect(serverList, asyncHandler)
	panicOn(err)
	p("nc client connection succeeded.")

	return &Sub{Nc: nc}
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

	// nats async handlers and options
	opts []nats.Option

	certs certConfig

	SkipTLS bool

	NatsName string
}

func (cfg *NatsConfig) SetupNatsConfig(name string) {

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
