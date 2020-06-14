package routers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/snowlyg/EasyDarwin/extend/db"
	"github.com/snowlyg/EasyDarwin/models"
	"github.com/snowlyg/EasyDarwin/rtsp"
)

/**
 * @api {get} /api/v1/stream/add 启动拉转推
 * @apiGroup stream
 * @apiName StreamAdd
 * @apiParam {String} url RTSP源地址
 * @apiParam {String} [customPath] 转推时的推送PATH
 * @apiParam {String=TCP,UDP} [transType=TCP] 拉流传输模式
 * @apiParam {Number} [idleTimeout] 拉流时的超时时间
 * @apiParam {Number} [heartbeatInterval] 拉流时的心跳间隔，毫秒为单位。如果心跳间隔不为0，那拉流时会向源地址以该间隔发送OPTION请求用来心跳保活
 * @apiSuccess (200) {String} ID	拉流的ID。后续可以通过该ID来停止拉流
 */
func (h *APIHandler) StreamAdd(c *gin.Context) {
	type Form struct {
		Id                uint   `form:"id" `
		URL               string `form:"source" binding:"required"`
		CustomPath        string `form:"customPath"`
		TransType         string `form:"transType"`
		IdleTimeout       int    `form:"idleTimeout"`
		HeartbeatInterval int    `form:"heartbeatInterval"`
	}
	var form Form
	err := c.Bind(&form)
	if err != nil {
		log.Printf("Pull to push err:%v", err)
		return
	}

	// save to db.
	oldStream := models.Stream{}
	if db.SQLite.Where("id = ? ", form.Id).First(&oldStream).RecordNotFound() {
		stream := models.Stream{
			URL:               form.URL,
			RealURl:           form.URL,
			CustomPath:        form.CustomPath,
			TransType:         form.TransType,
			IdleTimeout:       form.IdleTimeout,
			HeartbeatInterval: form.HeartbeatInterval,
			Status:            false,
		}
		db.SQLite.Create(&stream)
		c.IndentedJSON(200, stream)
	} else {
		oldStream.URL = form.URL
		oldStream.RealURl = form.URL
		oldStream.CustomPath = form.CustomPath
		oldStream.IdleTimeout = form.IdleTimeout
		oldStream.TransType = form.TransType
		oldStream.HeartbeatInterval = form.HeartbeatInterval
		oldStream.Status = false
		db.SQLite.Save(oldStream)
		c.IndentedJSON(200, oldStream)
	}

}

/**
 * @apiDefine stream 流管理
 */

/**
 * @api {get} /api/v1/stream/start 启动拉转推
 * @apiGroup stream
 * @apiName StreamStart
 * @apiParam {String} url RTSP源地址
 * @apiParam {String} [customPath] 转推时的推送PATH
 * @apiParam {String=TCP,UDP} [transType=TCP] 拉流传输模式
 * @apiParam {Number} [idleTimeout] 拉流时的超时时间
 * @apiParam {Number} [heartbeatInterval] 拉流时的心跳间隔，毫秒为单位。如果心跳间隔不为0，那拉流时会向源地址以该间隔发送OPTION请求用来心跳保活
 * @apiSuccess (200) {String} ID	拉流的ID。后续可以通过该ID来停止拉流
 */
func (h *APIHandler) StreamStart(c *gin.Context) {
	type Form struct {
		ID string `form:"id" binding:"required"`
	}
	var form Form
	err := c.Bind(&form)
	if err != nil {
		log.Printf("Pull to push err:%v", err)
		return
	}

	if err = startStream(form.ID); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, fmt.Sprintf("start pull to push err:%v", err))
		return
	}

	c.IndentedJSON(200, "OK")
	return
}

func startStream(id string) error {
	stream := models.GetStream(id)
	client, err := GetClient(stream.URL, stream.CustomPath, stream.TransType, stream.HeartbeatInterval)
	if err != nil {
		return err
	}

	pusher := rtsp.NewClientPusher(client)
	if rtsp.GetServer().GetPusher(pusher.Path()) != nil {
		pusher = rtsp.GetServer().GetPusher(pusher.Path())
	}

	err = client.Start(time.Duration(stream.IdleTimeout) * time.Second)
	if err != nil {
		if len(rtsp.NewPath) > 0 {
			stream.RealURl = rtsp.NewPath
			client.URL = rtsp.NewPath
			rtsp.NewPath = ""

			err = client.Start(time.Duration(stream.IdleTimeout) * time.Second)
			if err != nil {
				log.Printf("Pull stream err :%v", err)
				return err
			}
			db.SQLite.Model(&stream).Update(map[string]interface{}{"PusherId": pusher.ID(), "Status": true})
			if stream.Status {
				rtsp.GetServer().AddPusher(pusher)
				return nil
			}
			return errors.New(fmt.Sprintf("Start fail"))

		} else {
			log.Printf("Pull stream err :%v", err)
			return err
		}
	} else {
		db.SQLite.Model(&stream).Update(map[string]interface{}{"PusherId": pusher.ID(), "Status": true})
		if stream.Status {
			rtsp.GetServer().AddPusher(pusher)
			return nil
		}
		return errors.New(fmt.Sprintf("Start fail"))

	}

}

/**
 * @api {post} /api/v1/stream/startAll 批量启动推流
 * @apiGroup stream
 * @apiName StreamStartAll
 * @apiParam {String} ids 拉流的IDs
 * @apiUse simpleSuccess
 */
func (h *APIHandler) StreamStartAll(c *gin.Context) {

	type Form struct {
		Ids string `form:"ids" binding:"required"`
	}

	var form Form
	err := c.Bind(&form)
	if err != nil {
		log.Printf("stop pull to push err:%v", err)
		c.AbortWithStatusJSON(http.StatusBadRequest, fmt.Sprintf("stop pull to push err:%v", err))
		return
	}

	ids := strings.Split(strings.Replace(form.Ids, "\"", "", -1), ",")
	for _, id := range ids {
		if err = startStream(id); err != nil {
			log.Printf("stop pull to push err:%v", err)
			continue
		}
	}

	c.IndentedJSON(200, "OK")
	return
}

func GetClient(url, customPath, fTransType string, heartbeatInterval int) (*rtsp.RTSPClient, error) {
	agent := fmt.Sprintf("EasyDarwinGo/%s", BuildVersion)
	if BuildDateTime != "" {
		agent = fmt.Sprintf("%s(%s)", agent, BuildDateTime)
	}
	client, err := rtsp.NewRTSPClient(rtsp.GetServer(), url, int64(heartbeatInterval)*1000, agent)
	if err != nil {
		return nil, err
	}
	if customPath != "" && !strings.HasPrefix(customPath, "/") {
		customPath = "/" + customPath
	}
	client.CustomPath = customPath

	client.TransType = GetTransType(fTransType)

	return client, nil
}

func GetTransType(fTransType string) rtsp.TransType {
	var cTransType rtsp.TransType
	switch strings.ToLower(fTransType) {
	case "udp":
		cTransType = rtsp.TRANS_TYPE_UDP
	case "tcp":
		fallthrough
	default:
		cTransType = rtsp.TRANS_TYPE_TCP
	}

	return cTransType
}

/**
 * @api {get} /api/v1/stream/stop 停止推流
 * @apiGroup stream
 * @apiName StreamStop
 * @apiParam {String} id 拉流的ID
 * @apiUse simpleSuccess
 */
func (h *APIHandler) StreamStop(c *gin.Context) {
	type Form struct {
		ID string `form:"id" binding:"required"`
	}
	var form Form
	err := c.Bind(&form)
	if err != nil {
		log.Printf("stop pull to push err:%v", err)
		return
	}

	isStop := stopStream(form.ID)
	if isStop {
		c.IndentedJSON(200, "OK")
		return
	}

	c.AbortWithStatusJSON(http.StatusBadRequest, fmt.Sprintf("Stop stream fail %v", form.ID))
}

func stopStream(id string) bool {

	stream := models.GetStream(id)

	pusher := rtsp.GetServer().GetPusher(stream.CustomPath)
	if pusher != nil {
		pusher.RTSPClient.Stoped = true
		rtsp.GetServer().RemovePusher(pusher)
	} else {
		return false
	}

	stream.PusherId = ""
	stream.Status = false
	db.SQLite.Save(&stream)
	if !stream.Status {
		log.Printf("Stop %v success ", stream.URL)
		return true
	}

	return false
}

/**
 * @api {post} /api/v1/stream/stopAll 批量停止推流
 * @apiGroup stream
 * @apiName StreamStop
 * @apiParam {String} ids 拉流的IDs
 * @apiUse simpleSuccess
 */
func (h *APIHandler) StreamStopAll(c *gin.Context) {

	type Form struct {
		Ids string `form:"ids" binding:"required"`
	}

	var form Form
	err := c.Bind(&form)
	if err != nil {
		log.Printf("stop pull to push err:%v", err)
		c.AbortWithStatusJSON(http.StatusBadRequest, fmt.Sprintf("stop pull to push err:%v", err))
		return
	}

	ids := strings.Split(strings.Replace(form.Ids, "\"", "", -1), ",")
	for _, id := range ids {
		stopStream(id)
	}

	c.IndentedJSON(200, "OK")
	log.Printf("Stop success ")
	return
}

/**
 * @api {get} /api/v1/stream/del 删除推流
 * @apiGroup stream
 * @apiName StreamDel
 * @apiParam {String} id 拉流的ID
 * @apiUse simpleSuccess
 */
func (h *APIHandler) StreamDel(c *gin.Context) {

	type Form struct {
		ID string `form:"id" binding:"required"`
	}

	var form Form
	err := c.Bind(&form)
	if err != nil {
		log.Printf("stop pull to push err:%v", err)
		return
	}

	stream := models.GetStream(form.ID)

	db.SQLite.Unscoped().Delete(stream)

}
