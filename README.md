# EasyDarwin开源流媒体服务器

这是一个 EasyDarwin 的 fork 分支，主要是修复了某些资源推流的时候会出现 302 重定向的问题。

同时对推流页面做了一些小修改，增加了推流列表的启动和停止管理。

很可惜，此项目在推流电信 iptv 资源的表现不好，主要是 iptv 的资源，SDP 协议采用的是 MP2T/AVP 传输类型。

我采用了另一种方案，使用 FFMPEG + RTMP 或者 FFMPEG + RTSP 甚至是 FFMPEG+ HLS 的方式实现 IPTV 资源的推流。

我用 EasyDarwin 改写了一个项目用于管理 FFMPEG 命令 ：[https://github.com/snowlyg/GoEasyFfmpeg](https://github.com/snowlyg/GoEasyFfmpeg),此项目只用于管理 FFMPEG 命令
所有你需要另外部署一个 RTMP 或者 RTSP 服务，可以使用 [https://github.com/gwuhaolin/livego](https://github.com/gwuhaolin/livego),或者 nignx 搭建。
当然，你还需要安装 FFMPEG。

#### 演示地址：（服务器有点垃圾，加载比较慢）账号/密码：admin/admin
http://www.snowlyg.com:10008/



###### 如果有疑问可以提交 issues,或者加群咨询 irisgo 交流群 ：676717248
<a target="_blank" href="//shang.qq.com/wpa/qunwpa?idkey=cc99ccf86be594e790eacc91193789746af7df4a88e84fe949e61e5c6d63537c"><img border="0" src="http://pub.idqqimg.com/wpa/images/group.png" alt="Iris-go" title="Iris-go"></a>
