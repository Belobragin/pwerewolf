package config

import (
	"errors"
	"os"

	"github.com/ardanlabs/conf"
	wfc "github.com/belobragin/bvv_utils/goutils/config"
	"github.com/belobragin/werewolf/bus/internal/config/storage"
)

// CloudResnetConfig: the resnet lambda lives in its own cfn stack; the predict path is RESNET_PREDICT_PATH.
type CloudResnetConfig struct {
	ResnetStackName string `conf:"env:AWS_RESNET_CFN_STACK_NAME,default:crresnet-bvv-cf-resource"`
}

func (c *CloudResnetConfig) GetResnetStackName() string {
	return c.ResnetStackName
}

type CloudConfig struct {
	*wfc.MetricConfig
	*wfc.StandardConfig
	*wfc.CorsConfig
	*wfc.SqsConfigStatic
	*wfc.ResnetConfig
	CloudResnetConfig
	storage.StorageCloudConfig
}

func NewCloudConfig() (*CloudConfig, error) {
	var (
		e   error
		err []error
		c   = new(CloudConfig)
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
	if e = c.SqsConfigStatic.ValidateSqsConfig(); e != nil {
		err = append(err, e)
	}
	// host and port are not used in the cloud: the url comes from the stack output
	if c.ResnetStackName == "" || c.ResnetConfig == nil || c.GetResnetPredictPath() == "" {
		err = append(err, errors.New("resnet config data invalid: AWS_RESNET_CFN_STACK_NAME and RESNET_PREDICT_PATH are required"))
	}
	if e = c.ValidateStorageConfig(); e != nil {
		err = append(err, e)
	}
	return c, errors.Join(err...)
}
