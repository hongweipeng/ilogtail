package othttp

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/alibaba/ilogtail"
	"github.com/alibaba/ilogtail/pkg/logger"
	"github.com/alibaba/ilogtail/pkg/protocol"
	"google.golang.org/protobuf/proto"

	converter "github.com/alibaba/ilogtail/pkg/protocol/converter"
	otlpv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	logv1 "go.opentelemetry.io/proto/otlp/logs/v1"
)

type FlusherStdout struct {
	RemoteURL string
	client    *http.Client
	converter *converter.Converter
	context   ilogtail.Context
	queue     chan *otlpv1.ExportLogsServiceRequest // 上传队列
	counter   sync.WaitGroup
}

// 初始化
func (p *FlusherStdout) Init(context ilogtail.Context) error {
	p.context = context
	if p.RemoteURL == "" {
		err := errors.New("remoteURL is empty")
		logger.Error(p.context.GetRuntimeContext(), "FLUSHER_INIT_ALARM", "http flusher init fail, error", err)
		return err
	}

	p.client = &http.Client{
		Timeout: time.Second * 5,
	}
	converter, err := converter.NewConverter(converter.ProtocolOtlpLogV1, converter.EncodingNone, nil, nil)
	if err != nil {
		logger.Error(p.context.GetRuntimeContext(), "FLUSHER_INIT_ALARM", "http flusher init fail, error", err)
		return err
	}
	p.converter = converter

	p.queue = make(chan *otlpv1.ExportLogsServiceRequest)
	for i := 0; i < 4; i++ {
		// n 个上报协程
		go p.runTask()
	}

	return nil
}

func (*FlusherStdout) Description() string {
	return "ot http flusher for logtail"
}

// Flush the logGroup list to stdout or files.
func (p *FlusherStdout) Flush(projectName string, logstoreName string, configName string, logGroupList []*protocol.LogGroup) error {
	request := p.convertLogGroupToRequest(logGroupList)
	p.addTask(request)
	return nil
}

func (p *FlusherStdout) flushWithRetry(ot_request *otlpv1.ExportLogsServiceRequest) error {
	defer p.countDownTask()
	var err error
	max_retry_times := 5
	data, err := proto.Marshal(ot_request)

	if err != nil {
		log.Fatal("marshaling error: ", err)
		return err
	}
	logger.Info(p.context.GetRuntimeContext(), "upload", ot_request.String())
	for i := 0; i <= max_retry_times; i++ {
		ok, e := p.doPost(data)
		if ok || e == nil {
			break
		}
		err = e
		<-time.After(30 * time.Second)
	}
	// converter.PutPooledByteBuf(&data)
	return err
}

func (p *FlusherStdout) doPost(data []byte) (bool, error) {
	url := p.RemoteURL // todo: 从配置文件中获取

	request, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))

	//request.Header.Add("Authorization", "xxxx")

	response, err := p.client.Do(request)
	if err != nil {
		logger.Error(p.context.GetRuntimeContext(), "post error: ", err)
		return false, err
	}
	defer response.Body.Close()
	return true, nil
}

// 转换为 ot proto
func (f *FlusherStdout) convertLogGroupToRequest(logGroupList []*protocol.LogGroup) *otlpv1.ExportLogsServiceRequest {
	resourceLogs := make([]*logv1.ResourceLogs, 0)
	for _, logGroup := range logGroupList {
		c, _ := f.converter.Do(logGroup)
		if log, ok := c.(*logv1.ResourceLogs); ok {
			resourceLogs = append(resourceLogs, log)
		}
	}
	return &otlpv1.ExportLogsServiceRequest{
		ResourceLogs: resourceLogs,
	}
}

func (p *FlusherStdout) addTask(pbData *otlpv1.ExportLogsServiceRequest) {
	p.counter.Add(1)
	p.queue <- pbData
}

func (p *FlusherStdout) runTask() {
	for pbData := range p.queue {
		p.flushWithRetry(pbData)
	}
}

func (p *FlusherStdout) countDownTask() {
	p.counter.Done()
}

func (p *FlusherStdout) SetUrgent(flag bool) {
}

// IsReady is ready to flush
func (*FlusherStdout) IsReady(projectName string, logstoreName string, logstoreKey int64) bool {
	return true
}

// Stop ...
func (p *FlusherStdout) Stop() error {
	p.counter.Wait()
	close(p.queue)
	return nil
}

// 注册插件
func init() {
	ilogtail.Flushers["flusher_othttp"] = func() ilogtail.Flusher {
		return &FlusherStdout{}
	}
}
