package routers

import (
	"fmt"
	"github.com/snowlyg/EasyDarwin/extend/db"
	"github.com/snowlyg/EasyDarwin/models"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/snowlyg/EasyDarwin/extend/utils"
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

	var streams []models.Stream
	if err := db.SQLite.Find(&streams).Error; err != nil {
		log.Printf("find stream err:%v", err)
		return
	}

	hostname := utils.GetRequestHostname(c.Request)
	pushers := make([]interface{}, 0)
	for _, stream := range streams {
		elems := map[string]interface{}{
			"id":         stream.ID,
			"pusherId":   stream.PusherId,
			"source":     stream.URL,
			"customPath": stream.CustomPath,
			"transType":  stream.TransType,
			"status":     "已停止",
		}

		getPushers := rtsp.Instance.GetPushers()

		for _, pusher := range getPushers {
			if pusher.Path() == stream.CustomPath {
				if stream.Status {
					if !pusher.RTSPClient.Stoped {
						elems["status"] = "已启动"
					}
				}
				port := pusher.Server().TCPPort
				url := fmt.Sprintf("rtsp://%s:%d%s", hostname, port, pusher.Path())
				if port == 554 {
					url = fmt.Sprintf("rtsp://%s%s", hostname, pusher.Path())
				}
				if form.Q != "" && !strings.Contains(strings.ToLower(url), strings.ToLower(form.Q)) {
					continue
				}

				elems["pusherId"] = pusher.ID()
				elems["url"] = url
				elems["path"] = pusher.Path()
				//if len(pusher.TransType())>0{
				//	elems["transType"] = pusher.TransType()
				//}
				elems["inBytes"] = pusher.InBytes()
				elems["outBytes"] = pusher.OutBytes()
				elems["startAt"] = utils.DateTime(pusher.StartAt())
				elems["onlines"] = len(pusher.GetPlayers())
			}
		}

		pushers = append(pushers, elems)
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
