package realization

import (
	"crypto/rsa"
	"errors"
	"net/http"
	"time"

	"github.com/belobragin/bvv_utils/goutils/abroker"
	su "github.com/belobragin/bvv_utils/goutils/usecase"
	"github.com/belobragin/werewolf/bus/adapter"
	"github.com/belobragin/werewolf/bus/internal/config"
	"github.com/belobragin/werewolf/bus/pkg/database"
	"github.com/belobragin/werewolf/bus/pkg/storage"
	"github.com/belobragin/werewolf/bus/port"
	"github.com/belobragin/werewolf/bus/usecase"

	"go.uber.org/zap"
)

const (
	projectHttpClientTimeout = time.Second * 240
)

type UseCaseRealization struct {
	*su.StandardUseCaseRealization
	*su.StandardCorsRealization
	db          database.ProjectDbaseI
	st          storage.ProjectStorageI
	key         *rsa.PublicKey
	conf        config.ConfigI
	nb          abroker.AllBrokerI
	debug       bool
	resnet      su.ResnetPredictI
	fooFromPath func(*http.Request, string) (*string, error)
}

func NewUseCaseRealization(
	zapstruct *zap.Logger,
	conf config.ConfigI,
	serviceName string,
	db database.ProjectDbaseI,
	nb abroker.AllBrokerI,
	st storage.ProjectStorageI,
	resnet su.ResnetPredictI,
) (*UseCaseRealization, error) {
	var (
		err []error
		s   = new(UseCaseRealization)
		// p   = make(map[error]int)
	)
	s.debug = conf.GetDebug()
	s.conf = conf
	errC := make(chan error, 1)
	if st, e := su.NewStandardUseCase(conf, serviceName, zapstruct, errC); e != nil {
		err = append(err, e)
	} else {
		s.StandardUseCaseRealization = st
	}
	if sc, e := su.NewStandardCors(conf); e != nil {
		err = append(err, e)
	} else {
		s.StandardCorsRealization = sc
	}
	s.db = db
	s.nb = nb
	s.st = st
	s.resnet = resnet
	// s.resnetPredictApi = conf.GetResnetPredictApi()
	// s.resnetPredictApi = fmt.Sprintf("http://%s:%s/%s", conf.GetResnetHost(), conf.GetResnetPort(), strings.TrimPrefix(conf.GetResnetPredictPath(), "/"))
	if len(err) != 0 {
		return nil, errors.Join(err...)
	}
	s.fooFromPath = GetObjectIDFromPath
	return s, nil
}

func (s *UseCaseRealization) GetDb() database.ProjectDbaseI {
	return s.db
}
func (s *UseCaseRealization) GetStorage() storage.ProjectStorageI {
	return s.st
}
func (s *UseCaseRealization) CloseDb() error {
	return s.db.CloseDb()
}
func (s *UseCaseRealization) GetVersion() string {
	return s.db.GetVersion()
}
func (s *UseCaseRealization) GetBroker() abroker.AllBrokerI {
	return s.nb
}
func (s *UseCaseRealization) GetFooPath() func(*http.Request, string) (*string, error) {
	return s.fooFromPath
}
func (s *UseCaseRealization) GetDebug() bool {
	return s.debug
}
func (s *UseCaseRealization) GetResnet() su.ResnetPredictI {
	return s.resnet
}

func (s *UseCaseRealization) IntroduceApiClient() (usecase.ApiClientI, error) {
	var (
		a   = new(adapter.ApiClientRealization)
		err []error
	)

	a.Debug = s.GetDebug()
	a.Zapstruct = s.GetLog()
	a.HttpClient = &http.Client{
		Timeout:   projectHttpClientTimeout,
		Transport: s.resnet.GetTransport(),
		// Transport: &middleware.AuthRoundTripper{
		// 	Zapstruct: s.GetLog(),
		// 	Proxied:   http.DefaultTransport,
		// },
	}
	a.Resnet = s.resnet

	if smApi, e := su.NewMetricRealization(s.GetMetricPort(), su.NewApiCallMetrica); e != nil {
		err = append(err, e)
	} else {
		a.ApiMetric = smApi
	}
	a.Db = s.GetDb()
	a.Storage = s.GetStorage()
	a.Broker = s.GetBroker()
	a.FooFromPath = s.GetFooPath()
	if len(err) != 0 {
		return nil, errors.Join(err...)
	}
	return a, nil
}

// The clock runs from the accepted connection; an ssh tunnel opens it before the request travels the network.
const ServerReadHeaderTimeout = 5 * time.Second

// run server:
func (s *UseCaseRealization) NewApiServer() *http.Server {
	srv, err := port.NewMuxProjectRouter(s)
	if err != nil {
		return nil
	}
	return &http.Server{
		Addr:              ":" + s.GetApiPort(),
		ReadHeaderTimeout: ServerReadHeaderTimeout,
		Handler:           srv,
	}
}

// run prometheus metrics server:
func (s *UseCaseRealization) NewMetricServer() *http.Server {
	srv, err := port.NewMetricProjectRouter(s)
	if err != nil {
		return nil
	}
	return &http.Server{
		Addr:              ":" + s.GetMetricPort(),
		ReadHeaderTimeout: ServerReadHeaderTimeout,
		Handler:           srv,
	}
}
