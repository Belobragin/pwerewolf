package config

import (
	"errors"
	"os"

	"github.com/ardanlabs/conf"
	wfc "github.com/belobragin/bvv_utils/goutils/config"
	"github.com/belobragin/werewolf/bus/internal/config/storage"
)

type LocalConfig struct {
	*wfc.MetricConfig
	*wfc.StandardConfig
	*wfc.CorsConfig
	*wfc.BrokerConfigStatic
	*wfc.ResnetConfig
	storage.StorageLocalConfig
}

func NewLocalConfig() (*LocalConfig, error) {
	var (
		e   error
		err []error
		c   = new(LocalConfig)
	)

	if e = conf.Parse(os.Args, "", c); e != nil {
		err = append(err, e)
	}
	if e = c.MetricConfig.ValidateMetricConfig(); e != nil {
		err = append(err, e)
	}
	if e = c.StandardConfig.ValidateStandardConfig(); e != nil {
		err = append(err, e)
	}
	if e = c.CorsConfig.ValidateCorsConfig(); e != nil {
		err = append(err, e)
	}
	if e = c.BrokerConfigStatic.ValidateBrokerConfig(); e != nil {
		err = append(err, e)
	}
	if e = c.ValidateResnetConfig(); e != nil {
		err = append(err, e)
	}
	if e = c.ValidateStorageConfig(); e != nil {
		err = append(err, e)
	}
	return c, errors.Join(err...)
}
