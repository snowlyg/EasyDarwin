package routers

import (
	"fmt"
	"github.com/penggy/EasyGoLib/db"
	"github.com/snowlyg/EasyDarwin/models"
	"log"
	"strings"
	//"strings"

	"github.com/gin-gonic/gin"
	"github.com/penggy/EasyGoLib/utils"
	"github.com/snowlyg/EasyDarwin/rtsp"
)

/**
 * @apiDefine stats 统计
 */

/**
 * @apiDefine playerInfo
 * @apiSuccess (200) {String} rows.id
 * @apiSuccess (200) {String} rows.path
 * @apiSuccess (200) {String} rows.transType 传输模式
 * @apiSuccess (200) {Number} rows.inBytes 入口流量
 * @apiSuccess (200) {Number} rows.outBytes 出口流量
 * @apiSuccess (200) {String} rows.startAt 开始时间
 */

/**
 * @api {get} /api/v1/pushers 获取推流列表
 * @apiGroup stats
 * @apiName Pushers
 * @apiParam {Number} [start] 分页开始,从零开始
 * @apiParam {Number} [limit] 分页大小
 * @apiParam {String} [sort] 排序字段
 * @apiParam {String=ascending,descending} [order] 排序顺序
 * @apiParam {String} [q] 查询参数
 * @apiSuccess (200) {Number} total 总数
 * @apiSuccess (200) {Array} rows 推流列表
 * @apiSuccess (200) {String} rows.id
 * @apiSuccess (200) {String} rows.path
 * @apiSuccess (200) {String} rows.transType 传输模式
 * @apiSuccess (200) {Number} rows.inBytes 入口流量
 * @apiSuccess (200) {Number} rows.outBytes 出口流量
 * @apiSuccess (200) {String} rows.startAt 开始时间
 * @apiSuccess (200) {Number} rows.onlines 在线人数
 */
func (h *APIHandler) Pushers(c *gin.Context) {

	form := utils.NewPageForm()
	if err := c.Bind(form); err != nil {
		return
	}

	hostname := utils.GetRequestHostname(c.Request)

	var streams []models.Stream
	if err := db.SQLite.Find(&streams).Error; err != nil {
		log.Printf("find stream err:%v", err)
		return
	}

	pushers := make([]interface{}, 0)
	for _, stream := range streams {
		var url string
		var path string
		var transType string
		var inBytes int
		var outBytes int
		var startAt string
		var onlines int
		statusText := "已停止"

		rIPushers := rtsp.Instance.GetPushers()
		for _, pusher := range rIPushers {
			port := pusher.Server().TCPPort

			rtsp := fmt.Sprintf("rtsp://%s:%d%s", hostname, port, pusher.Path())
			if port == 554 {
				rtsp = fmt.Sprintf("rtsp://%s%s", hostname, pusher.Path())
			}

			if form.Q != "" && !strings.Contains(strings.ToLower(rtsp), strings.ToLower(form.Q)) {
				continue
			}

			if stream.URL == pusher.RTSPClient.URL {
				if stream.Status {
					if !pusher.Stoped() {
						statusText = "已启动"
					}
				}
				startAtTime := utils.DateTime(pusher.StartAt())
				startAt = startAtTime.String()
				url, path, transType, inBytes, outBytes, onlines = rtsp, pusher.Path(), pusher.TransType(), pusher.InBytes(), pusher.OutBytes(), len(pusher.GetPlayers())
			}
		}

		pushers = append(pushers, map[string]interface{}{
			"id":        stream.ID,
			"streamId":  stream.StreamId,
			"url":       url,
			"path":      path,
			"source":    stream.URL,
			"transType": transType,
			"inBytes":   inBytes,
			"outBytes":  outBytes,
			"startAt":   startAt,
			"onlines":   onlines,
			"status":    statusText,
		})

	}

	pr := utils.NewPageResult(pushers)
	if form.Sort != "" {
		pr.Sort(form.Sort, form.Order)
	}

	pr.Slice(form.Start, form.Limit)
	c.IndentedJSON(200, pr)
}

/**
 * @api {get} /api/v1/players 获取拉流列表
 * @apiGroup stats
 * @apiName Players
 * @apiParam {Number} [start] 分页开始,从零开始
 * @apiParam {Number} [limit] 分页大小
 * @apiParam {String} [sort] 排序字段
 * @apiParam {String=ascending,descending} [order] 排序顺序
 * @apiParam {String} [q] 查询参数
 * @apiSuccess (200) {Number} total 总数
 * @apiSuccess (200) {Array} rows 推流列表
 * @apiSuccess (200) {String} rows.id
 * @apiSuccess (200) {String} rows.path
 * @apiSuccess (200) {String} rows.transType 传输模式
 * @apiSuccess (200) {Number} rows.inBytes 入口流量
 * @apiSuccess (200) {Number} rows.outBytes 出口流量
 * @apiSuccess (200) {String} rows.startAt 开始时间
 */
func (h *APIHandler) Players(c *gin.Context) {
	form := utils.NewPageForm()
	if err := c.Bind(form); err != nil {
		return
	}
	players := make([]*rtsp.Player, 0)
	for _, pusher := range rtsp.Instance.GetPushers() {
		for _, player := range pusher.GetPlayers() {
			players = append(players, player)
		}
	}
	hostname := utils.GetRequestHostname(c.Request)
	_players := make([]interface{}, 0)
	for i := 0; i < len(players); i++ {
		player := players[i]
		port := player.Server.TCPPort
		rtsp := fmt.Sprintf("rtsp://%s:%d%s", hostname, port, player.Path)
		if port == 554 {
			rtsp = fmt.Sprintf("rtsp://%s%s", hostname, player.Path)
		}
		_players = append(_players, map[string]interface{}{
			"id":        player.ID,
			"path":      rtsp,
			"transType": player.TransType.String(),
			"inBytes":   player.InBytes,
			"outBytes":  player.OutBytes,
			"startAt":   utils.DateTime(player.StartAt),
		})
	}
	pr := utils.NewPageResult(_players)
	if form.Sort != "" {
		pr.Sort(form.Sort, form.Order)
	}
	pr.Slice(form.Start, form.Limit)
	c.IndentedJSON(200, pr)
}
