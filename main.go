package main

import (
	"flag"
	"fmt"
	figure "github.com/common-nighthawk/go-figure"
	"github.com/snowlyg/EasyDarwin/extend/EasyGoLib/utils"
	"github.com/snowlyg/EasyDarwin/routers"
	"github.com/snowlyg/EasyDarwin/rtsp"
	"gopkg.in/alecthomas/kingpin.v2"
	"log"
	"os"
	"regexp"
	"strings"
)

var (
	gitCommitCode string
	buildDateTime string
)
var Version = "v0.0.0"

func newProgram(sargs []string) (*rtsp.Program, error) {
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

	args := rtsp.Args{
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

	protocols := make(map[rtsp.StreamProtocol]struct{})
	for _, proto := range strings.Split(args.ProtocolsStr, ",") {
		switch proto {
		case "udp":
			protocols[rtsp.STREAM_PROTOCOL_UDP] = struct{}{}

		case "tcp":
			protocols[rtsp.STREAM_PROTOCOL_TCP] = struct{}{}

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
	p := &rtsp.Program{
		HttpPort:  httpPort,
		Args:      args,
		Protocols: protocols,
	}

	var err error

	p.UdplRtp, err = rtsp.NewServerUdpListener(p, args.RtpPort, rtsp.TRACK_FLOW_RTP)
	if err != nil {
		return nil, err
	}

	p.UdplRtcp, err = rtsp.NewServerUdpListener(p, args.RtcpPort, rtsp.TRACK_FLOW_RTCP)
	if err != nil {
		return nil, err
	}

	p.Tcpl, err = rtsp.NewServerTcpListener(p)
	if err != nil {
		return nil, err
	}

	go p.UdplRtp.Run()
	go p.UdplRtcp.Run()
	go p.Tcpl.Run()
	go p.StartHTTP()

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

	_, err := newProgram(os.Args[1:])
	if err != nil {
		log.Fatal("ERR: ", err)
	}

	infty := make(chan struct{})
	<-infty

	figure.NewFigure("EasyDarwin", "", false).Print()
}
