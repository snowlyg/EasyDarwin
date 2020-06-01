package main

import (
	"context"
	"flag"
	"fmt"
	figure "github.com/common-nighthawk/go-figure"
	"github.com/kardianos/service"
	"github.com/snowlyg/EasyDarwin/extend/EasyGoLib/db"
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

type trackFlow int

const (
	_TRACK_FLOW_RTP trackFlow = iota
	_TRACK_FLOW_RTCP
)

type track struct {
	rtpPort  int
	rtcpPort int
}

type streamProtocol int

const (
	_STREAM_PROTOCOL_UDP streamProtocol = iota
	_STREAM_PROTOCOL_TCP
)

func (s streamProtocol) String() string {
	if s == _STREAM_PROTOCOL_UDP {
		return "udp"
	}
	return "tcp"
}

type args struct {
	version      bool
	protocolsStr string
	rtspPort     int
	rtpPort      int
	rtcpPort     int
	readTimeout  time.Duration
	writeTimeout time.Duration
	publishUser  string
	publishPass  string
	readUser     string
	readPass     string
	preScript    string
	postScript   string
}

type program struct {
	protocols  map[streamProtocol]struct{}
	args       args
	HttpPort   int
	HttpServer *http.Server
	rtspServer *rtsp.Server
	tcpl       *serverTcpListener
	udplRtp    *serverUdpListener
	udplRtcp   *serverUdpListener
}

// StopHTTP 停止 http
func (p *program) StopHTTP() (err error) {
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
func (p *program) StartHTTP() (err error) {
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

func (p *program) Start(s service.Service) (err error) {
	log.Println("********** START **********")
	if utils.IsPortInUse(p.HttpPort) {
		err = fmt.Errorf("HTTP port[%d] In Use", p.HttpPort)
		return
	}

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
			err = p.StopRTSP()
			if err != nil {
				return
			}
			err = p.StopHTTP()
			if err != nil {
				return
			}

			utils.ReloadConf()

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

	go func() {
		log.Printf("demon pull streams")
		for {
			var streams []models.Stream
			if err := db.SQLite.Find(&streams).Error; err != nil {
				log.Printf("find stream err:%v", err)
				return
			}

			for i := len(streams) - 1; i > -1; i-- {
				v := streams[i]
				pusher := rtsp.NewClientPusher(v.ID, v.URL, v.CustomPath)
				if rtsp.GetServer().GetPusher(v.CustomPath) != nil {
					continue
				}
				if v.Status {
					pusher.Stoped = false
					rtsp.GetServer().AddPusher(pusher)
				}
				//streams = streams[0:i]
				//streams = append(streams[:i], streams[i+1:]...)
			}
			time.Sleep(1 * time.Second)
		}
	}()
	log.Printf("server start ")

	return
}

// StartRTSP 启动 rtsp
func (p *program) StartRTSP() (err error) {
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
func (p *program) StopRTSP() (err error) {
	if p.rtspServer == nil {
		err = fmt.Errorf("RTSP Server Not Found")
		return
	}
	p.rtspServer.Stop()
	return
}

// Stop 停止服务
func (p *program) Stop(s service.Service) (err error) {
	defer log.Println("********** STOP **********")
	defer utils.CloseLogWriter()

	err = p.StopRTSP()
	if err != nil {
		log.Println("start http server error", err)
	}
	err = p.StopHTTP()
	if err != nil {
		log.Println("start http server error", err)
	}

	models.Close()
	return
}

func newProgram(sargs []string) (*program, error) {
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

	args := args{
		version:      *argVersion,
		protocolsStr: *argProtocolsStr,
		rtspPort:     *argRtspPort,
		rtpPort:      *argRtpPort,
		rtcpPort:     *argRtcpPort,
		readTimeout:  *argReadTimeout,
		writeTimeout: *argWriteTimeout,
		publishUser:  *argPublishUser,
		publishPass:  *argPublishPass,
		readUser:     *argReadUser,
		readPass:     *argReadPass,
		preScript:    *argPreScript,
		postScript:   *argPostScript,
	}

	if args.version == true {
		fmt.Println(Version)
		os.Exit(0)
	}

	protocols := make(map[streamProtocol]struct{})
	for _, proto := range strings.Split(args.protocolsStr, ",") {
		switch proto {
		case "udp":
			protocols[_STREAM_PROTOCOL_UDP] = struct{}{}

		case "tcp":
			protocols[_STREAM_PROTOCOL_TCP] = struct{}{}
		default:
			return nil, fmt.Errorf("unsupported protocol: %s", proto)
		}
	}

	if len(protocols) == 0 {
		return nil, fmt.Errorf("no protocols provided")
	}
	if (args.rtpPort % 2) != 0 {
		return nil, fmt.Errorf("rtp port must be even")
	}
	if args.rtcpPort != (args.rtpPort + 1) {
		return nil, fmt.Errorf("rtcp and rtp ports must be consecutive")
	}
	if args.publishUser != "" {
		if !regexp.MustCompile("^[a-zA-Z0-9]+$").MatchString(args.publishUser) {
			return nil, fmt.Errorf("publish username must be alphanumeric")
		}
	}
	if args.publishPass != "" {
		if !regexp.MustCompile("^[a-zA-Z0-9]+$").MatchString(args.publishPass) {
			return nil, fmt.Errorf("publish password must be alphanumeric")
		}
	}
	if args.readUser != "" && args.readPass == "" || args.readUser == "" && args.readPass != "" {
		return nil, fmt.Errorf("read username and password must be both filled")
	}
	if args.readUser != "" {
		if !regexp.MustCompile("^[a-zA-Z0-9]+$").MatchString(args.readUser) {
			return nil, fmt.Errorf("read username must be alphanumeric")
		}
	}
	if args.readPass != "" {
		if !regexp.MustCompile("^[a-zA-Z0-9]+$").MatchString(args.readPass) {
			return nil, fmt.Errorf("read password must be alphanumeric")
		}
	}
	if args.readUser != "" && args.readPass == "" || args.readUser == "" && args.readPass != "" {
		return nil, fmt.Errorf("read username and password must be both filled")
	}

	log.Printf("rtsp-simple-server %s", Version)
	httpPort := utils.Conf().Section("http").Key("port").MustInt(10008)
	rtspServer := rtsp.GetServer()

	p := &program{
		protocols:  protocols,
		args:       args,
		HttpPort:   httpPort,
		rtspServer: rtspServer,
	}
	var err error
	p.udplRtp, err = newServerUdpListener(p, args.rtpPort, _TRACK_FLOW_RTP)
	if err != nil {
		return nil, err

	}

	p.udplRtcp, err = newServerUdpListener(p, args.rtcpPort, _TRACK_FLOW_RTCP)
	if err != nil {
		return nil, err
	}

	p.tcpl, err = newServerTcpListener(p)
	if err != nil {
		return nil, err
	}

	go p.udplRtp.run()
	go p.udplRtcp.run()
	go p.tcpl.run()

	return p, nil
}

func (p *program) close() {
	p.tcpl.close()
	p.udplRtcp.close()
	p.udplRtp.close()
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
