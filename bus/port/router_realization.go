package port

import (
	"net/http"
	"regexp"

	m "github.com/belobragin/bvv_utils/goutils/middleware"
	su "github.com/belobragin/bvv_utils/goutils/usecase"
	"github.com/belobragin/werewolf/bus/internal/project_middleware"
	"github.com/belobragin/werewolf/bus/usecase"
	"github.com/rs/cors"
	"go.uber.org/zap"
)

func NewMetricProjectRouter(s usecase.UseCaseI) (http.Handler, error) {
	mux := http.NewServeMux()
	l, err := s.IntroduceApiClient()
	if err != nil {
		s.GetLog().Error("Metric client can not be introduced", zap.Error(err))
		return nil, err
	}
	su.AddCustomMetricRoutes(mux, l.GetApiMetric())
	return mux, nil
}

func NewMuxProjectRouter(s usecase.UseCaseI) (http.Handler, error) {

	zapstruct := s.GetLog()
	l, err := s.IntroduceApiClient()
	if err != nil {
		zapstruct.Error("Logic client can not be introduced", zap.Error(err))
		return nil, err
	}
	router := ProjectRouter
	router.NotFoundHandler = m.NotFound(zapstruct, project_middleware.NotFound, l)

	// business-api: must run first, it sets wfDocsRouteApiName used below
	AddNewWerewolfRoutes(l)

	router.Use(
		m.RequestIDMiddleware,
	)

	if s.GetUseCors() {
		crs := cors.New(cors.Options{
			AllowOriginFunc: func(origin string) bool {
				rex, err := regexp.Compile(s.GetAllowOrigin())
				if err != nil {
					s.GetLog().Warn("[HTTP] Regular expression is not valnewfavorite_id: %s, err: %s",
						zap.String("AllowOrigin", s.GetAllowOrigin()),
						zap.String("Cors error", err.Error()))
					return false
				}
				return rex.MatchString(origin)
			},
			AllowedMethods: []string{
				http.MethodGet,
				http.MethodPost,
				http.MethodPatch,
				http.MethodDelete,
			},
			AllowedHeaders:       []string{"Content-Type", "Authorization", "request-startTime"},
			ExposedHeaders:       []string{"link"},
			MaxAge:               s.GetMaxAge(),
			OptionsSuccessStatus: http.StatusOK,
			AllowCredentials:     false,
		})
		return crs.Handler(router), nil
	}
	return router, nil
}
