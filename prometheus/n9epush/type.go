package n9epush

import (
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

// 数据模型
//{
//        "nid": "24",
//        "metric": "test.buss_n_service_n",
//        "endpoint": "ip_3.n",
//        "tags": "app_name=/buss_1/buss_2/test.service_n",
//        "value": 15.4,
//        "timestamp": 1628217974,
//        "step": 20,
//        "counterType": "GAUGE"
//    }

// Metrics 定义，Bu, project, App 需要上传
// Routine，Nid，PushGateway 不传按默认
type Metrics struct {
	Registry    prometheus.Registry  `json:"registry"`
	Collector   prometheus.Collector `json:"collector"`

	Bu          string              `json:"bu"`
	Project     string              `json:"project"`
	App         string              `json:"app"`

	Routine     time.Time           `json:"routine,omitempty"`
	Nid         string              `json:"nid,omitempty"`
	PushGateway string              `json:"pushgateway,omitempty"`
	GlobalTags  []string            `json:"globaltags,omitempty"`
}

type N9EMetrics struct {
	Metric      string              `json:"metric"`
	Endpoint    string              `json:"endpoint,omitempty"`
	Timestamp   int64               `json:"timestamp,omitempty"`
	Step        int64               `json:"step"`
	Value       float64             `json:"value"`
	CounterType Countertype         `json:"counterType"`
	Tags        interface{}         `json:"tags,omitempty"`
	TagsMap     map[string]string   `json:"tagsMap,omitempty"`
	Extra       string              `json:"extra,omitempty"`
}

type Countertype struct {
	Gauge         prometheus.Gauge
	Counter       prometheus.Counter
	// Subtract      prometheus.Gauge
	Histogram     prometheus.Histogram
	Summary       prometheus.Summary
	GaugeOpts     prometheus.GaugeOpts
	CounterOpts   prometheus.CounterOpts
	SummaryOpts   prometheus.SummaryOpts
	HistogramOpts prometheus.HistogramOpts
}

type DefaultMode struct {
	IsFailureCodes      bool                    `json:"isfailurecodes"`
	UserDuration        prometheus.HistogramVec `json:"userduration"`
	UserRequest         prometheus.CounterVec   `json:"userrequest"`
	UserFails           prometheus.CounterVec   `json:"userfails"`
	ReturnCode          prometheus.CounterVec   `json:"returncode,omitempty"`
	Codes               map[string]string       `json:"codes,omitempty"`
}

func NewRequest() *prometheus.CounterVec {
	userRequestSuccess = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name: "_user_requests_total",
			Help: "Number of success requests",
		},
		[]string{},
	)
	return userRequestSuccess
}

func NewFails() *prometheus.CounterVec {
	userRequestFails = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name: "_user_fails_total",
			Help: "Number of fails requests",
		},
		[]string{},
	)
	return userRequestFails
}

func NewDuration() *prometheus.HistogramVec {
	userDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: subsystem,
			Name: "_user_request_duration",
			Help: "Duration of request",
		},
		[]string{},
	)
	return userDuration
}

func NewRetCode() *prometheus.CounterVec {
	returnCode = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: subsystem,
			Name: "_return_code",
			Help: "return code",
		},
		[]string{"code"},
	)
	return returnCode
}

func NewDefaultMode() {

}
