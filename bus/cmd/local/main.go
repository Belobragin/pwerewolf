//	@title			werwolf service API
//	@version		0.0.1
//	@description	API documentation

//	@contact.name	bvv
//	@contact.url	http://www.swagger.io/support
//	@contact.email	vblvbl133@gmail.com

//	@license.name				Apache 2.0
//	@license.url				http://www.apache.org/licenses/LICENSE-2.0.html
//	@query.collection.format	multi

//	@host		localhost:8101
//	@basePath	/api/v1
//	@schemes	http

//	@query.collection.format	multi

package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/belobragin/bvv_utils/goutils/abroker"
	"github.com/belobragin/bvv_utils/goutils/log"
	"github.com/belobragin/bvv_utils/goutils/mistake"
	"github.com/belobragin/bvv_utils/goutils/util"
	"github.com/belobragin/werewolf/bus/internal/config"
	"github.com/belobragin/werewolf/bus/usecase"
	"github.com/belobragin/werewolf/bus/usecase/local"
	"github.com/belobragin/werewolf/bus/usecase/realization"

	"go.uber.org/zap"
)

const (
	werewolfServiceName = "werewolf"
)

var (
	err           error
	shutdown      = make(chan os.Signal, 1)
	metricsErrors = make(chan error, 1)
	eventErrors   = make(chan error, 1)
	serverErrors  = make(chan error, 1)
	sch           = make(chan struct{}, 1)
)

func init() {
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
}

func main() {
	useCase, errChM, err := injectLocalDependency()
	if err != nil {
		panic(err)
	}
	var (
		zapstruct     = useCase.GetLog()
		allStopch     = useCase.GetAllStopChan()
		httpServer    = useCase.NewApiServer()
		metricsServer = useCase.NewMetricServer()
		broker        = useCase.GetBroker()
	)
	defer func() {
		err = useCase.GetBroker().CloseBroker()
		if err != nil {
			zapstruct.Error("Error broker unsubscribe", zap.Error(err))
		}
		err = useCase.GetDb().CloseDb()
		if err != nil {
			zapstruct.Error("Error closing database", zap.Error(err))
		}
	}()

	// run goroutines error processor:
	go mistake.ErrorsProcessor(
		zapstruct,
		sch,
		errChM..., // errors from go routines
	)
	// metrics and telemetry:
	go func() {
		metricsErrors <- metricsServer.ListenAndServe()
	}()
	// metrics server:
	zapstruct.Info("Metric server started, listening on ", zap.String("port:", useCase.GetMetricPort()))

	// message events:
	go func() {
		broker.ListenEvent(eventErrors, sch)
	}()
	// main server:
	zapstruct.Info("Broker started")

	// rest api server:
	go func() {
		serverErrors <- httpServer.ListenAndServe()
	}()
	// main server:
	zapstruct.Info("Http server started, listening on ", zap.String("port:", useCase.GetApiPort()))

	select {
	case <-sch:
		util.GoodbyeHttp(zapstruct, allStopch, metricsServer, httpServer)
	case err = <-metricsErrors:
		util.Halt(zapstruct, "Metric server crashed", err, sch)
	case err = <-eventErrors:
		util.Halt(zapstruct, "Event processor crashed", err, sch)
	case err = <-serverErrors:
		util.Halt(zapstruct, "Http server crashed", err, sch)
	case sig := <-shutdown:
		zapstruct.Info("Shutdown %v started", zap.Reflect("signal", sig))
		defer zapstruct.Info("Shutdown complete", zap.Reflect("signal", sig))
		util.Halt(zapstruct, "os signal", nil, sch)
	}
}

func injectLocalDependency() (
	usecase.UseCaseI,
	[]<-chan error,
	error,
) {
	var (
		zapstruct = new(zap.Logger)
	)
	// config:
	cf, err := config.NewLocalConfig()
	if err != nil {
		return nil, nil, err
	}
	if cf.LogLevel != "" {
		zapstruct = log.StructuredInit(cf.GetLogLevel(), "", nil)
	} else {
		return nil, nil, mistake.ErrNullLogger
	}
	localDb, err := local.BuildGormDbS()
	if err != nil {
		return nil, nil, err
	}
	natsBroker, err := abroker.NewNatsBroker(
		cf.GetNatsUri(), cf.GetNatsName(), cf.GetNatsPort(), cf.GetNatsMetricPort(),
		cf.GetNatsUser(), cf.GetNatsPsw(),
		cf.GetListenSubject(), cf.GetPublishSubject(),
		local.NewProcessMessage(localDb, zapstruct),
		zapstruct,
	)
	if err != nil {
		return nil, nil, err
	}
	storage, err := local.NewLocalStorage(cf)
	if err != nil {
		return nil, nil, err
	}
	projectResnet, err := local.NewResnetLocalRealization(*cf, zapstruct)
	if err != nil {
		return nil, nil, err
	}
	// usecase:
	useCase, err := realization.NewUseCaseRealization(
		zapstruct,
		cf,
		werewolfServiceName,
		localDb,
		natsBroker,
		storage,
		projectResnet,
	)
	if err != nil {
		return nil, nil, err
	}
	// change capacity if project addds more err channels:
	m := make([]<-chan error, 0, 1)
	m = append(m, useCase.ErrCh())

	return useCase, m, nil
}
