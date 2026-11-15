package local

import (
	"testing"

	"github.com/belobragin/werewolf/bus/pkg/model/diagnose_model"
)

func newTxDb(t *testing.T) *DbRealizationLocal {
	t.Helper()
	d, err := BuildGormDbS()
	if err != nil {
		t.Skipf("database is not available: %v", err)
	}
	tx := d.Db.DB.Begin()
	if tx.Error != nil {
		t.Skipf("database is not available: %v", tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	d.Db.DB = tx
	return d
}

func prediction(label string) diagnose_model.ResnetPredictionData {
	return diagnose_model.ResnetPredictionData{ClassLabel: label, ClassName: label, ClassID: 1, Probability: 1}
}
