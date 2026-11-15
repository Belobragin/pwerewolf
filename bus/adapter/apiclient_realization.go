package adapter

import (
	"bytes"
	"context"
	"net/http"

	"github.com/belobragin/bvv_utils/goutils/abroker"
	"github.com/belobragin/bvv_utils/goutils/mistake"
	su "github.com/belobragin/bvv_utils/goutils/usecase"
	"github.com/belobragin/werewolf/bus/pkg/database"
	"github.com/belobragin/werewolf/bus/pkg/storage"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// structure:
type ApiClientRealization struct {
	Db          database.ProjectDbaseI
	Storage     storage.ProjectStorageI
	Broker      abroker.AllBrokerI
	Zapstruct   *zap.Logger
	Debug       bool
	ApiMetric   su.MetricI
	EventMetric su.MetricI
	Resnet      su.ResnetPredictI
	FooFromPath func(*http.Request, string) (*string, error)
	HttpClient  *http.Client
}

// private methods
func (s *ApiClientRealization) getHttpClient() HttpClientI {
	return s.HttpClient
}

// interface realization
func (l *ApiClientRealization) GetDb() database.ProjectDbaseI {
	return l.Db
}
func (l *ApiClientRealization) GetBroker() abroker.AllBrokerI {
	return l.Broker
}
func (l *ApiClientRealization) GetStorage() storage.ProjectStorageI {
	return l.Storage
}
func (l *ApiClientRealization) GetResnet() su.ResnetPredictI {
	return l.Resnet
}
func (l *ApiClientRealization) GetLog() *zap.Logger {
	return l.Zapstruct
}

func (l *ApiClientRealization) GetDebug() bool {
	return l.Debug
}

func (l *ApiClientRealization) GetApiMetric() su.MetricI {
	return l.ApiMetric
}

func (l *ApiClientRealization) GetEventMetric() su.MetricI {
	return l.EventMetric
}

func (l *ApiClientRealization) Do(r *http.Request) (*http.Response, error) {
	return l.getHttpClient().Do(r)
}

func (l *ApiClientRealization) GetObjectIDFromPath(r *http.Request, objectId string) (*string, error) {
	return l.FooFromPath(r, objectId)
}

func (l *ApiClientRealization) NewRequestWithContext(
	ctx context.Context, method string, url string, body []byte) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
}
func (l *ApiClientRealization) GetFromPath(r *http.Request, s string) (*string, error) {
	return l.FooFromPath(r, s)
}

func (l *ApiClientRealization) SetStatusCodeApiCallMetrica(s su.ServiceApiCallMetricaLabelsI) error {
	if l.GetApiMetric() == nil {
		return mistake.ErrApiMetricVectorNull
	}
	if len(l.GetApiMetric().GetCounterVec()) == 0 {
		return mistake.ErrEventMetricVectorNull
	}
	am := l.GetApiMetric().GetCounterVec()[0]
	_, e := am.GetMetricWithLabelValues(s.GetStatusCodeLabel(), s.GetMethodLabel(), s.GetModellabel())
	if e != nil {
		return mistake.NewAddErr(mistake.ErrMetricaLabel, e)
	}
	am.With(prometheus.Labels{
		"code":   s.GetStatusCodeLabel(),
		"method": s.GetMethodLabel(),
		"model":  s.GetModellabel()},
	).Inc()

	return nil
}

func (s *ApiClientRealization) GetHttpClient() *http.Client {
	return s.HttpClient
}
