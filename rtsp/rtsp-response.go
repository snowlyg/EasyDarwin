package rtsp

import (
	"fmt"
	"strconv"
)

type Response struct {
	Version    string
	StatusCode int
	Status     string
	Header     map[string]interface{}
	Body       string
}

// NewResponse 新建响应
func NewResponse(statusCode int, status, cSeq, sid, body string) *Response {
	res := &Response{
		Version:    RTSP_VERSION,
		StatusCode: statusCode,
		Status:     status,
		Header:     map[string]interface{}{"CSeq": cSeq, "Session": sid},
		Body:       body,
	}
	len := len(body)
	if len > 0 {
		res.Header["Content-Length"] = strconv.Itoa(len)
	} else {
		delete(res.Header, "Content-Length")
	}
	return res
}

//  String 响应
func (r *Response) String() string {
	str := fmt.Sprintf("%s %d %s\r\n", r.Version, r.StatusCode, r.Status)
	for key, value := range r.Header {
		str += fmt.Sprintf("%s: %s\r\n", key, value)
	}
	str += "\r\n"
	str += r.Body
	return str
}

//  SetBody 设置主体内容
func (r *Response) SetBody(body string) {
	len := len(body)
	r.Body = body
	if len > 0 {
		r.Header["Content-Length"] = strconv.Itoa(len)
	} else {
		delete(r.Header, "Content-Length")
	}
}
