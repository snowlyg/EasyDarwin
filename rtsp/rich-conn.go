package rtsp

import (
	"net"
	"time"
)

type RichConn struct {
	net.Conn
	timeout time.Duration
}

// 读取
func (conn *RichConn) Read(b []byte) (n int, err error) {
	if conn.timeout > 0 {
		_ = conn.Conn.SetReadDeadline(time.Now().Add(conn.timeout))
	} else {
		var t time.Time
		_ = conn.Conn.SetReadDeadline(t)
	}
	return conn.Conn.Read(b)
}

// Write 写入
func (conn *RichConn) Write(b []byte) (n int, err error) {
	if conn.timeout > 0 {
		_ = conn.Conn.SetWriteDeadline(time.Now().Add(conn.timeout))
	} else {
		var t time.Time
		_ = conn.Conn.SetWriteDeadline(t)
	}
	return conn.Conn.Write(b)
}
