package local

import (
	"net/http"

	"fmt"
	"strings"

	"github.com/belobragin/bvv_utils/goutils/middleware"
	"github.com/belobragin/werewolf/bus/internal/config"
	"go.uber.org/zap"
)

type ResnetLocalRealization struct {
	resnetHost        string
	resnetPort        string
	resnetPredictPath string
	logger            *zap.Logger
}

func (r *ResnetLocalRealization) GetPredictApiUrl() string {
	// s.resnetPredictApi =
	return fmt.Sprintf("http://%s:%s/%s", r.resnetHost, r.resnetPort, strings.TrimPrefix(r.resnetPredictPath, "/"))
}

func (r *ResnetLocalRealization) GetTransport() http.RoundTripper {
	return &middleware.AuthRoundTripper{
		Zapstruct: r.logger,
		Proxied:   http.DefaultTransport,
	}
}

func NewResnetLocalRealization(cf config.LocalConfig, w *zap.Logger) (*ResnetLocalRealization, error) {

	return &ResnetLocalRealization{
		resnetHost:        cf.GetResnetHost(),
		resnetPort:        cf.GetResnetPort(),
		resnetPredictPath: cf.GetResnetPredictPath(),
		logger:            w,
	}, nil
}
