package adapter

import (
	"context"
	"net/http"

	sf "github.com/belobragin/bvv_utils/goutils/filter"
	"github.com/belobragin/werewolf/bus/internal/pdata"
	"github.com/belobragin/werewolf/bus/pkg/filter"
)

func (l *ApiClientRealization) ListImage(ctx context.Context, f sf.ProjectFilterI) (any, error) {
	if _, err := f.DistillateQuery(); err != nil {
		return nil, err
	}
	imageFilter, ok := f.(*filter.ImageFilter)
	if !ok || imageFilter == nil {
		return nil, pdata.ErrInvalidListFilter
	}

	output, err := l.GetDb().DbListImage(ctx, *imageFilter)
	if err != nil {
		return nil, err
	}

	output.Code = http.StatusOK

	return any(*output), nil
}
