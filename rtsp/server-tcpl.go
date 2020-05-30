package rtsp

import (
	"log"
	"net"
	"sync"

	"github.com/aler9/gortsplib"
)

type ServerTcpListener struct {
	Server     *Server
	Nconn      *net.TCPListener
	Mutex      sync.RWMutex
	Clients    map[*serverClient]struct{}
	Publishers map[string]*serverClient
	Done       chan struct{}
}

func NewServerTcpListener(p *Server) (*ServerTcpListener, error) {
	nconn, err := net.ListenTCP("tcp", &net.TCPAddr{
		Port: p.Args.RtspPort,
	})
	if err != nil {
		return nil, err
	}

	l := &ServerTcpListener{
		Server:     p,
		Nconn:      nconn,
		Clients:    make(map[*serverClient]struct{}),
		Publishers: make(map[string]*serverClient),
		Done:       make(chan struct{}),
	}

	l.log("opened on :%d", p.Args.RtspPort)
	return l, nil
}

func (l *ServerTcpListener) log(format string, args ...interface{}) {
	log.Printf("[TCP listener] "+format, args...)
}

func (l *ServerTcpListener) Run() {
	for {
		nconn, err := l.Nconn.AcceptTCP()
		if err != nil {
			break
		}

		NewServerClient(l.Server, nconn)
	}

	// close clients
	var doneChans []chan struct{}
	func() {
		l.Mutex.Lock()
		defer l.Mutex.Unlock()
		for c := range l.Clients {
			c.Close()
			doneChans = append(doneChans, c.done)
		}
	}()
	for _, c := range doneChans {
		<-c
	}

	close(l.Done)
}

func (l *ServerTcpListener) Close() {
	l.Nconn.Close()
	<-l.Done
}

func (l *ServerTcpListener) forwardTrack(path string, id int, flow TrackFlow, frame []byte) {
	for c := range l.Clients {
		if c.path == path && c.state == _CLIENT_STATE_PLAY {
			if c.streamProtocol == STREAM_PROTOCOL_TCP {
				if flow == TRACK_FLOW_RTP {
					l.Server.UdplRtp.Write <- &udpWrite{
						addr: &net.UDPAddr{
							IP:   c.ip(),
							Zone: c.zone(),
							Port: c.streamTracks[id].RtpPort,
						},
						buf: frame,
					}
				} else {
					l.Server.UdplRtp.Write <- &udpWrite{
						addr: &net.UDPAddr{
							IP:   c.ip(),
							Zone: c.zone(),
							Port: c.streamTracks[id].RtcpPort,
						},
						buf: frame,
					}
				}

			} else {
				c.write <- &gortsplib.InterleavedFrame{
					Channel: trackToInterleavedChannel(id, flow),
					Content: frame,
				}
			}
		}
	}
}
