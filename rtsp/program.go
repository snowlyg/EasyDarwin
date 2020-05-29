package rtsp

import (
	"context"
	"fmt"
	"github.com/kardianos/service"
	"github.com/snowlyg/EasyDarwin/extend/EasyGoLib/utils"
	"github.com/snowlyg/EasyDarwin/models"
	"github.com/snowlyg/EasyDarwin/routers"
	"log"
	"net/http"
	"time"
)

type Args struct {
	Version      bool
	ProtocolsStr string
	RtspPort     int
	RtpPort      int
	RtcpPort     int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PublishUser  string
	PublishPass  string
	ReadUser     string
	ReadPass     string
	PreScript    string
	PostScript   string
}

type TrackFlow int

const (
	TRACK_FLOW_RTP TrackFlow = iota
	TRACK_FLOW_RTCP
)

type Track struct {
	RtpPort  int
	RtcpPort int
}

type StreamProtocol int

const (
	STREAM_PROTOCOL_UDP StreamProtocol = iota
	STREAM_PROTOCOL_TCP
)

func (s StreamProtocol) String() string {
	if s == STREAM_PROTOCOL_UDP {
		return "udp"
	}
	return "tcp"
}

type Program struct {
	Protocols  map[StreamProtocol]struct{}
	Args       Args
	HttpPort   int
	HttpServer *http.Server
	Tcpl       *ServerTcpListener
	UdplRtp    *ServerUdpListener
	UdplRtcp   *ServerUdpListener
}

// StopHTTP 停止 http
func (p *Program) StopHTTP() (err error) {
	if p.HttpServer == nil {
		err = fmt.Errorf("HTTP Server Not Found")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = p.HttpServer.Shutdown(ctx); err != nil {
		return
	}
	return
}

// StartHTTP 启动 http
func (p *Program) StartHTTP() (err error) {
	p.HttpServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", p.HttpPort),
		Handler:           routers.Router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	link := fmt.Sprintf("http://%s:%d", utils.LocalIP(), p.HttpPort)
	log.Println("http server start -->", link)
	go func() {
		if err := p.HttpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Println("start http server error", err)
		}
		log.Println("http server end")
	}()
	return
}

// Stop 停止服务
func (p *Program) Stop(s service.Service) (err error) {
	defer log.Println("********** STOP **********")
	defer utils.CloseLogWriter()
	_ = p.StopHTTP()
	models.Close()
	return
}
