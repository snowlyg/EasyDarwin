package main

import (
	"log"
	"net"
	"time"
)

type udpWrite struct {
	addr *net.UDPAddr
	buf  []byte
}

type ServerUdpListener struct {
	P     *Program
	Nconn *net.UDPConn
	Flow  TrackFlow
	Write chan *udpWrite
	Done  chan struct{}
}

func NewServerUdpListener(p *Program, port int, flow TrackFlow) (*ServerUdpListener, error) {
	nconn, err := net.ListenUDP("udp", &net.UDPAddr{
		Port: port,
	})
	if err != nil {
		return nil, err
	}

	l := &ServerUdpListener{
		P:     p,
		Nconn: nconn,
		Flow:  flow,
		Write: make(chan *udpWrite),
		Done:  make(chan struct{}),
	}

	l.log("opened on :%d", port)
	return l, nil
}

func (l *ServerUdpListener) log(format string, args ...interface{}) {
	var label string
	if l.Flow == TRACK_FLOW_RTP {
		label = "RTP"
	} else {
		label = "RTCP"
	}
	log.Printf("[UDP/"+label+" listener] "+format, args...)
}

func (l *ServerUdpListener) Start() {
	go func() {
		for w := range l.Write {
			l.Nconn.SetWriteDeadline(time.Now().Add(l.P.Args.WriteTimeout))
			l.Nconn.WriteTo(w.buf, w.addr)
		}
	}()

	for {
		// create a buffer for each read.
		// this is necessary since the buffer is propagated with channels
		// so it must be unique.
		buf := make([]byte, 2048) // UDP MTU is 1400
		n, addr, err := l.Nconn.ReadFromUDP(buf)
		if err != nil {
			break
		}

		func() {
			l.P.Tcpl.Mutex.RLock()
			defer l.P.Tcpl.Mutex.RUnlock()

			// find path and track id from ip and port
			path, trackId := func() (string, int) {
				for _, pub := range l.P.Tcpl.Publishers {
					for i, t := range pub.streamTracks {
						if !pub.ip().Equal(addr.IP) {
							continue
						}

						if l.Flow == TRACK_FLOW_RTP {
							if t.RtpPort == addr.Port {
								return pub.path, i
							}
						} else {
							if t.RtcpPort == addr.Port {
								return pub.path, i
							}
						}
					}
				}
				return "", -1
			}()
			if path == "" {
				return
			}

			l.P.Tcpl.forwardTrack(path, trackId, l.Flow, buf[:n])
		}()
	}

	close(l.Write)

	close(l.Done)
}

func (l *ServerUdpListener) Stop() {
	l.Nconn.Close()
	<-l.Done
}
