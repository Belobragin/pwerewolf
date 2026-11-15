package config

import (
	wfc "github.com/belobragin/bvv_utils/goutils/config"
)

type ConfigI interface {
	wfc.MetricConfigI
	wfc.StandardConfigI
	wfc.CorsConfigI
	wfc.ResnetConfigI
	StorageConfigI
}
