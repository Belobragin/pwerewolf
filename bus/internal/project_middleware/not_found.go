package project_middleware

import (
	"fmt"
	"net/http"

	"github.com/belobragin/bvv_utils/goutils/middleware"
	"github.com/belobragin/bvv_utils/goutils/mistake"
	su "github.com/belobragin/bvv_utils/goutils/usecase"
	"github.com/belobragin/werewolf/bus/usecase"
	"go.uber.org/zap"
)

func NotFound(a any) (http.Handler, error) {
	l, ok := a.(usecase.ApiClientI)
	if !ok {
		return nil, mistake.ErrIncorrectNotFoundHandler
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		zapstruct := l.GetLog()
		zapstruct.Error("Not found")
		h := &su.ServiceApiCallMetricaLabels{
			Model:  "route not found",
			Method: r.Method,
			Code:   http.StatusNotFound,
		}
		if err := l.GetApiMetric().SetStatusCodeApiCallMetrica(h); err != nil {
			zapstruct.Error("Error on metrica processing ", zap.Error(err))
		}
		middleware.StandardRespond(zapstruct, w, r, h.Code, fmt.Sprintf("%d page not_found %s", h.Code, r.URL.Path))
	}), nil
}
