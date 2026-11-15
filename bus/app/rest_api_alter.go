package app

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	m "github.com/belobragin/bvv_utils/goutils/middleware"
	metrica "github.com/belobragin/bvv_utils/goutils/usecase"
	"github.com/belobragin/werewolf/bus/internal/pdata"
	model "github.com/belobragin/werewolf/bus/pkg/model/alter_model"
	"github.com/google/uuid"
)

// AlterObjectHandler 	godoc
//
//	@Summary		Cast adversary image from the image
//	@Description	Cast adversary image from the image with the selected algorythm
//	@Accept			x-www-form-urlencoded
//	@Produce		json
//	@Param			objectID	path	string	true	"object_id"
//	@Param			InputAlter	body	model.InputAlter	true	"input alter data"
//	@Tags			alter
//	@Success		201	{object}	model.AlterPostReply
//	@Failure		400	{object}	model.AlterPostErr
//	@Failure		404	{object}	model.AlterPostErr
//	@Failure		409	{object}	model.AlterGetErr
//	@Failure		500	{object}	model.AlterPostErr
//	@Router			/werewolf/alter/{objectID} [post]
func (l *ApiClientStruct) AlterObjectHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err       error
		body      []byte
		req       model.InputAlter
		outErr    model.AlterPostErr
		o, output any
		hasErr    bool
		objectID  uuid.UUID
		h         = &metrica.ServiceApiCallMetricaLabels{
			Model:  "werewolf_alter_object_post",
			Method: r.Method,
		}
	)
	zapstruct := l.GetLog()
	id, err := l.GetObjectIDFromPath(r, "objectID")
	if err != nil {
		outErr.Code = http.StatusBadRequest
		hasErr = true
		goto out
	}
	if objectID, err = uuid.Parse(*id); err != nil {
		outErr.Code = http.StatusBadRequest
		hasErr = true
		goto out
	}
	outErr.ObjectID = objectID
	body, err = io.ReadAll(r.Body)
	if err != nil {
		outErr.Code = http.StatusBadRequest
		hasErr = true
		goto out
	}
	err = json.Unmarshal(body, &req)
	if err != nil {
		outErr.Code = http.StatusBadRequest
		hasErr = true
		goto out
	}
	o, err = l.PostAlterObject(r.Context(), objectID, &req)
	if err != nil {
		h.Code = pdata.GetCodeFromErr(err)
		outErr.Code = h.Code
		hasErr = true
	} else {
		h.Code = http.StatusCreated
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

// GetAdversarialResultbyImageIDHandler 	godoc
//
//	@Summary		Get adversary image data from the database
//	@Description	Get adversary image data from the database
//	@Accept			x-www-form-urlencoded
//	@Produce		json
//	@Param			objectID	path	string	true	"object_id"
//	@Tags			alter
//	@Success		200	{object}	model.AlterGetReply
//	@Failure		400	{object}	model.AlterGetErr
//	@Failure		404	{object}	model.AlterGetErr
//	@Failure		500	{object}	model.AlterGetErr
//	@Router			/werewolf/alter/{objectID} [get]
func (l *ApiClientStruct) GetAdversarialResultbyImageIDHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err       error
		outErr    model.AlterGetErr
		o, output any
		hasErr    bool
		objectID  uuid.UUID
		h         = &metrica.ServiceApiCallMetricaLabels{
			Model:  "werewolf_adversarial_object_get",
			Method: r.Method,
		}
	)
	zapstruct := l.GetLog()
	id, err := l.GetObjectIDFromPath(r, "objectID")
	if err != nil {
		outErr.Code = http.StatusBadRequest
		hasErr = true
		goto out
	}
	if objectID, err = uuid.Parse(*id); err != nil {
		outErr.Code = http.StatusBadRequest
		hasErr = true
		goto out
	}
	outErr.ObjectID = objectID
	o, err = l.GetAdversarialObject(r.Context(), objectID)
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
