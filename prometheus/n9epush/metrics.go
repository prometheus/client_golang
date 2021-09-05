package n9epush

import (
	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"os"
	"strconv"
	"time"
)

func init() {
	// 1. 获取环境中通用标签 Done
	// 2. 读取指定文件或者 pod ENV 环境 Done
	// 3. 解析出通用标签 Done
	// 4. 初始化指定特殊 Tag。项目配置或者自定义 Done
	// 5. 初始化时指定时延区间
	// 6. 定义 push 上报端口

	// 1. 定义指标名
	// 2. 内部指标处理
	// 3. 定义逻辑执行前时间
	// 4. 定义自定义 Tag, 非必要
	// 5. 执行业务代码
	//    添加自定义 tag 或者 不带

	// 请求数 Gauge
	// metricName: user_request
	// 返回码 tag: retcode=0, retcode=1, recode=-1...
	// 错误 tag: success=0, success=1
	// 调用类型 tag: type=rpc, type=internal
	// 业务自定义 tag: tag=tags
	// val: 周期内次数累加

	// 时延分布区间 Histogram
	// metricName: user_request__time.histogram
	// 返回码 tag: retcode=0, retcode=1, recode=-1...
	// 错误 tag: success=0, success=1
	// 调用类型 tag: type=rpc, type=internal
	// 业务自定义 tag: tag=tags
	// 区间:
	// 20,val1
	// 50,val2
	// 500,val3
	// 1000,val4
	// 3000,val5

	// 时延分布中位值 Summary
	// metricName: user_request__time.histogram
	// 返回码 tag: retcode=0, retcode=1, recode=-1...
	// 错误 tag: success=0, success=1
	// 调用类型 tag: type=rpc, type=internal
	// 业务自定义 tag: tag=tags
	// 区间:
	// 50,val1
	// 80,val2
	// 90,val3
	// 95,val4
	// 99,val5
}

var (
	subsystem = "rd"
	isFailure bool
	defaultPushGateway = "http://localhost:2080/v1/push"

	code = make(map[string]string)

	// 用户请求成功数
	userRequestSuccess = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name: "_user_requests_total",
			Help: "Number of success requests",
		},
		[]string{},
	)

	// 用户请求失败数
	userRequestFails = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name: "_user_fails_total",
			Help: "Number of fails requests",
		},
		[]string{},
	)

	// 用户时延
	userDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: subsystem,
			Name: "_user_request_duration",
			Help: "Duration of request",
		},
		[]string{},
	)

	// 返回码
	returnCode = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name: "_return_code",
			Help: "return code",
		},
		[]string{"code"},
	)
)

func Defaultmode(buckets []float64, isFailure bool, codes []string) {


	prometheus.MustRegister(userDuration)

	if len(codes) != 0 {
		for _, v := range codes {
			if _, ok := code[v]; !ok {
				code[v] = v
			}
		}
	}
}

func Use() {
	registry := prometheus.NewRegistry()
	collector := collectors.NewGoCollector()
	registry.MustRegister(collector)

	// registry.Gather()
}

// 新建 Metrics 方法
func NewMetrics(bu, project, appName string, globalTags map[string]string) (metrics *Metrics) {
	metrics.Registry = prometheus.Registry{}

	if bu == "" || project == "" || appName == "" {
		glog.Fatal("please check if bu or project or appname if it's empty")
		return &Metrics{}
	}

	tags := make(map[string]string)
	// collector := prometheus.NewGoCollector()
	// prometheus.MustRegister(collector)
	// 将用户自定义 tag 写入
	if len(globalTags) != 0 {
		for k, v := range globalTags {
			tags[k] = v
		}
	}
	// 强制写固定的bu, project, appname到全局 tags
	tags["bu"] = bu
	tags["project"] = project
	tags["app"] = appName
	// 获取 pod 名称
	podName := os.Getenv("MY_POD_NAME")
	if podName == "" {
		podName = strconv.Itoa(os.Getpid())
	}
	tags["pid"] = podName
	// set global tags TODO

	return metrics
}

// PushGateway 循环推送
func (m *Metrics) StartPushLoop(nid string, routine time.Duration, pg string) {
	// 如果 nid 未指定，推送 n9e 默认租户 0
	if nid == "" {
		nid = "0"
	}
	// 推送周期态最小为 1s
	if routine < 1 {
		routine = time.Second * 1
	}
	// pg 默认本机 2080 端口的 n9e
	if pg == "" {
		pg = defaultPushGateway
	}


}
