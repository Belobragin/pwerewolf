package app

import (
	"net/http"
	"time"

	sf "github.com/belobragin/bvv_utils/goutils/filter"
	m "github.com/belobragin/bvv_utils/goutils/middleware"
	metrica "github.com/belobragin/bvv_utils/goutils/usecase"
	"github.com/belobragin/werewolf/bus/internal/pdata"
	"github.com/belobragin/werewolf/bus/pkg/filter"
	model "github.com/belobragin/werewolf/bus/pkg/model/list_model"
)

// ListImageHandler 	godoc
//
//	@Summary		List images
//	@Description	List images, adversarial images and filter images based on 'AND' logic
//	@Accept			x-www-form-urlencoded
//	@Produce		json
//	@Param			f	query	filter.ImageFilter	true	"filter search for images"
//	@Tags			list
//	@Success		200	{object}	model.ListImageReply
//	@Failure		400	{object}	model.ListImageErr
//	@Failure		404	{object}	model.ListImageErr
//	@Failure		500	{object}	model.ListImageErr
//	@Router			/werewolf/list [get]
func (l *ApiClientStruct) ListImageHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err       error
		outErr    model.ListImageErr
		o, output any
		hasErr    bool
		h         = &metrica.ServiceApiCallMetricaLabels{
			Model:  "werewolf_list_image_get",
			Method: r.Method,
		}
		f sf.ProjectFilterI
	)
	zapstruct := l.GetLog()
	if f, err = filter.NewImageFilter(); err != nil {
		outErr.Code = http.StatusBadRequest
		hasErr = true
		goto out
	}
	if err = f.ParseQueries(r.URL.Query()); err != nil {
		outErr.Code = http.StatusBadRequest
		hasErr = true
		goto out
	}
	o, err = l.ListImage(r.Context(), f)
	if err != nil {
		h.Code = pdata.GetCodeFromErr(err)
		outErr.Code = h.Code
		hasErr = true
	} else {
		h.Code = http.StatusOK
	}
	if err := l.GetApiMetric().SetStatusCodeApiCallMetrica(h); err != nil {
		h.Code = pdata.GetCodeFromErr(err)
		outErr.Code = h.Code
		hasErr = true
	}
out:
	if hasErr {
		// early validation errors skip the code assignment above; status 0 would panic in WriteHeader
		if h.Code == 0 {
			h.Code = outErr.Code
		}
		outErr.ErrMessage = err.Error()
		outErr.CreatedAt = time.Now().Format(time.RFC3339Nano)
		output = any(outErr)
	} else {
		output = o
	}
	m.StandardRespond(zapstruct, w, r, h.Code, output)
}
