package usecase

import (
	"net/http"

	"github.com/belobragin/bvv_utils/goutils/abroker"
	co "github.com/belobragin/bvv_utils/goutils/config"
	su "github.com/belobragin/bvv_utils/goutils/usecase"
	"github.com/belobragin/werewolf/bus/pkg/database"
	"github.com/belobragin/werewolf/bus/pkg/storage"
)

type UseCaseI interface {
	su.StandardUseCaseI
	co.CorsConfigI
	// project constituent parts
	GetResnet() su.ResnetPredictI
	NewApiServer() *http.Server
	NewMetricServer() *http.Server
	IntroduceApiClient() (ApiClientI, error)
	GetBroker() abroker.AllBrokerI
	GetDb() database.ProjectDbaseI
	GetStorage() storage.ProjectStorageI
	GetFooPath() func(*http.Request, string) (*string, error)
}
