package main

import (
	"context"
	"flag"
	"fmt"
	figure "github.com/common-nighthawk/go-figure"
	"github.com/kardianos/service"
	"github.com/snowlyg/EasyDarwin/extend/EasyGoLib/utils"
	"github.com/snowlyg/EasyDarwin/models"
	"github.com/snowlyg/EasyDarwin/routers"
	"github.com/snowlyg/EasyDarwin/rtsp"
	"gopkg.in/alecthomas/kingpin.v2"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var (
	gitCommitCode string
	buildDateTime string
)
var Version = "v0.0.0"

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

type Program struct {
	Protocols  map[StreamProtocol]struct{}
	Args       Args
	HttpPort   int
	HttpServer *http.Server
	rtspServer *rtsp.Server
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

func (p *Program) Start(s service.Service) (err error) {
	log.Println("********** START **********")
	if utils.IsPortInUse(p.HttpPort) {
		err = fmt.Errorf("HTTP port[%d] In Use", p.HttpPort)
		return
	}

	p.UdplRtp.Start()
	p.UdplRtcp.Start()
	p.Tcpl.Start()

	err = models.Init()
	if err != nil {
		return
	}
	err = routers.Init()
	if err != nil {
		return
	}

	err = p.StartHTTP()
	if err != nil {
		return
	}
	err = p.StartRTSP()
	if err != nil {
		return
	}

	log.Println("log files -->", utils.LogDir())

	if !utils.Debug {
		log.Println("log files -->", utils.LogDir())
		log.SetOutput(utils.GetLogWriter())
	}
	go func() {
		for range routers.API.RestartChan {
			p.UdplRtcp.Stop()
			p.UdplRtp.Stop()
			p.Tcpl.Stop()
			err = p.StopRTSP()
			if err != nil {
				return
			}
			err = p.StopHTTP()
			if err != nil {
				return
			}

			utils.ReloadConf()
			p.UdplRtp.Start()
			p.UdplRtcp.Start()
			p.Tcpl.Start()

			err = p.StartHTTP()
			if err != nil {
				return
			}
			err = p.StartRTSP()

			if err != nil {
				return
			}
		}
	}()

	return
}

// StartRTSP 启动 rtsp
func (p *Program) StartRTSP() (err error) {
	if p.rtspServer == nil {
		err = fmt.Errorf("RTSP Server Not Found")
		return
	}
	go func() {
		if err := p.rtspServer.Start(); err != nil {
			log.Println("start rtsp server error", err)
		}
		log.Println("rtsp server start")
	}()
	return
}

// StopRTSP 停止 rtsp
func (p *Program) StopRTSP() (err error) {
	if p.rtspServer == nil {
		err = fmt.Errorf("RTSP Server Not Found")
		return
	}
	p.rtspServer.Stop()
	return
}

// Stop 停止服务
func (p *Program) Stop(s service.Service) (err error) {
	defer log.Println("********** STOP **********")
	defer utils.CloseLogWriter()
	_ = p.StopRTSP()
	_ = p.StopHTTP()

	p.UdplRtcp.Stop()
	p.UdplRtp.Stop()
	p.Tcpl.Stop()
	models.Close()
	return
}

func newProgram(sargs []string) (*Program, error) {
	kingpin.CommandLine.Help = "rtsp-simple-server " + Version + "\n\n" + "RTSP server."

	argVersion := kingpin.Flag("version", "print version").Bool()
	argProtocolsStr := kingpin.Flag("protocols", "supported protocols").Default("udp,tcp").String()
	argRtspPort := kingpin.Flag("rtsp-port", "port of the RTSP TCP listener").Default("8554").Int()
	argRtpPort := kingpin.Flag("rtp-port", "port of the RTP UDP listener").Default("8000").Int()
	argRtcpPort := kingpin.Flag("rtcp-port", "port of the RTCP UDP listener").Default("8001").Int()
	argReadTimeout := kingpin.Flag("read-timeout", "timeout for read operations").Default("5s").Duration()
	argWriteTimeout := kingpin.Flag("write-timeout", "timeout for write operations").Default("5s").Duration()
	argPublishUser := kingpin.Flag("publish-user", "optional username required to publish").Default("").String()
	argPublishPass := kingpin.Flag("publish-pass", "optional password required to publish").Default("").String()
	argReadUser := kingpin.Flag("read-user", "optional username required to read").Default("").String()
	argReadPass := kingpin.Flag("read-pass", "optional password required to read").Default("").String()
	argPreScript := kingpin.Flag("pre-script", "optional script to run on client connect").Default("").String()
	argPostScript := kingpin.Flag("post-script", "optional script to run on client disconnect").Default("").String()

	kingpin.MustParse(kingpin.CommandLine.Parse(sargs))

	args := Args{
		Version:      *argVersion,
		ProtocolsStr: *argProtocolsStr,
		RtspPort:     *argRtspPort,
		RtpPort:      *argRtpPort,
		RtcpPort:     *argRtcpPort,
		ReadTimeout:  *argReadTimeout,
		WriteTimeout: *argWriteTimeout,
		PublishUser:  *argPublishUser,
		PublishPass:  *argPublishPass,
		ReadUser:     *argReadUser,
		ReadPass:     *argReadPass,
		PreScript:    *argPreScript,
		PostScript:   *argPostScript,
	}

	if args.Version == true {
		fmt.Println(Version)
		os.Exit(0)
	}

	protocols := make(map[StreamProtocol]struct{})
	for _, proto := range strings.Split(args.ProtocolsStr, ",") {
		switch proto {
		case "udp":
			protocols[STREAM_PROTOCOL_UDP] = struct{}{}

		case "tcp":
			protocols[STREAM_PROTOCOL_TCP] = struct{}{}
		default:
			return nil, fmt.Errorf("unsupported protocol: %s", proto)
		}
	}

	if len(protocols) == 0 {
		return nil, fmt.Errorf("no protocols provided")
	}
	if (args.RtpPort % 2) != 0 {
		return nil, fmt.Errorf("rtp port must be even")
	}
	if args.RtcpPort != (args.RtpPort + 1) {
		return nil, fmt.Errorf("rtcp and rtp ports must be consecutive")
	}
	if args.PublishUser != "" {
		if !regexp.MustCompile("^[a-zA-Z0-9]+$").MatchString(args.PublishUser) {
			return nil, fmt.Errorf("publish username must be alphanumeric")
		}
	}
	if args.PublishPass != "" {
		if !regexp.MustCompile("^[a-zA-Z0-9]+$").MatchString(args.PublishPass) {
			return nil, fmt.Errorf("publish password must be alphanumeric")
		}
	}
	if args.ReadUser != "" && args.ReadPass == "" || args.ReadUser == "" && args.ReadPass != "" {
		return nil, fmt.Errorf("read username and password must be both filled")
	}
	if args.ReadUser != "" {
		if !regexp.MustCompile("^[a-zA-Z0-9]+$").MatchString(args.ReadUser) {
			return nil, fmt.Errorf("read username must be alphanumeric")
		}
	}
	if args.ReadPass != "" {
		if !regexp.MustCompile("^[a-zA-Z0-9]+$").MatchString(args.ReadPass) {
			return nil, fmt.Errorf("read password must be alphanumeric")
		}
	}
	if args.ReadUser != "" && args.ReadPass == "" || args.ReadUser == "" && args.ReadPass != "" {
		return nil, fmt.Errorf("read username and password must be both filled")
	}

	log.Printf("rtsp-simple-server %s", Version)
	httpPort := utils.Conf().Section("http").Key("port").MustInt(10008)
	rtspServer := rtsp.GetServer()

	p := &Program{
		Protocols:  protocols,
		Args:       args,
		HttpPort:   httpPort,
		rtspServer: rtspServer,
	}
	var err error
	p.UdplRtp, err = NewServerUdpListener(p, args.RtpPort, TRACK_FLOW_RTP)
	if err != nil {
		return nil, err

	}

	p.UdplRtcp, err = NewServerUdpListener(p, args.RtcpPort, TRACK_FLOW_RTCP)
	if err != nil {
		return nil, err
	}

	p.Tcpl, err = NewServerTcpListener(p)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func main() {

	flag.StringVar(&utils.FlagVarConfFile, "config", "", "configure file path")
	flag.Parse()

	// log
	log.SetPrefix("[EasyDarwin] ")
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	log.Printf("git commit code:%s", gitCommitCode)
	log.Printf("build date:%s", buildDateTime)

	routers.BuildVersion = fmt.Sprintf("%s.%s", routers.BuildVersion, gitCommitCode)
	routers.BuildDateTime = buildDateTime

	p, err := newProgram(os.Args[1:])
	if err != nil {
		log.Fatal("ERR: ", err)
	}
	sec := utils.Conf().Section("service")
	svcConfig := &service.Config{
		Name:        sec.Key("name").MustString("EasyDarwin_Service"),
		DisplayName: sec.Key("display_name").MustString("EasyDarwin_Service"),
		Description: sec.Key("description").MustString("EasyDarwin_Service"),
	}
	s, err := service.New(p, svcConfig)
	if err != nil {
		log.Println(err)
		utils.PauseExit()
	}

	figure.NewFigure("EasyDarwin", "", false).Print()
	if err = s.Run(); err != nil {
		log.Println(err)
		utils.PauseExit()
	}

	figure.NewFigure("EasyDarwin", "", false).Print()
}
