package app

import (
	"net/http"
	"time"

	m "github.com/belobragin/bvv_utils/goutils/middleware"
	mistake "github.com/belobragin/bvv_utils/goutils/mistake"
	metrica "github.com/belobragin/bvv_utils/goutils/usecase"
	"github.com/belobragin/werewolf/bus/internal/pdata"
	model "github.com/belobragin/werewolf/bus/pkg/model/diagnose_model"
	"github.com/google/uuid"
)

// PostDiagnoseObjectHandler 	godoc
//
//	@Summary		Classify image with ResNet50 neural net
//	@Description	Classify image by image name and folder with ResNet50 neural net
//	@Accept			x-www-form-urlencoded
//	@Produce		json
//	@Param			image_name	query	string	true	"image name of the file"
//	@Param			folder		query	string	true	"storage folder with the file"
//	@Tags			diagnose
//	@Success		201	{object}	model.DiagnosePostReply
//	@Failure		400	{object}	model.DiagnosePostErr
//	@Failure		404	{object}	model.DiagnosePostErr
//	@Failure		500	{object}	model.DiagnosePostErr
//	@Router			/werewolf/diagnose [post]
func (l *ApiClientStruct) PostDiagnoseObjectHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err                    error
		o, output              any
		outErr                 model.DiagnosePostErr
		hasErr                 bool
		imageName, imageFolder string
		h                      = &metrica.ServiceApiCallMetricaLabels{
			Model:  "werewolf_diagnose_object_post",
			Method: r.Method,
		}
	)
	zapstruct := l.GetLog()
	imageName = r.URL.Query().Get("image_name")
	imageFolder = r.URL.Query().Get("folder")
	if len(imageName) == 0 || len(imageFolder) == 0 {
		err = mistake.ErrRequiredUrlParameterAbsent
		outErr.Code = http.StatusBadRequest
		hasErr = true
		goto out
	}
	o, err = l.PostDiagnoseObject(r.Context(), imageName, imageFolder)
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

// PostDiagnoseObjectByIDHandler 	godoc
//
//	@Summary		Classify image with ResNet50 neural net
//	@Description	Classify image by image id (uuid) with ResNet50 neural net
//	@Accept			x-www-form-urlencoded
//	@Produce		json
//	@Param			objectID	path	string	true	"object_id"
//	@Tags			diagnose
//	@Success		201	{object}	model.DiagnosePostReply
//	@Failure		400	{object}	model.DiagnosePostErr
//	@Failure		404	{object}	model.DiagnosePostErr
//	@Failure		500	{object}	model.DiagnosePostErr
//	@Router			/werewolf/diagnose/{objectID} [post]
func (l *ApiClientStruct) PostDiagnoseObjectByIDHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err       error
		o, output any
		outErr    model.DiagnosePostErr
		hasErr    bool
		objectID  uuid.UUID
		h         = &metrica.ServiceApiCallMetricaLabels{
			Model:  "werewolf_diagnose_object_byid_post",
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
	outErr.ImageID = objectID
	o, err = l.PostDiagnoseObjectByID(r.Context(), objectID)
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

// GetDiagnoseResultHandler 	godoc
//
//	@Summary		Get classification result from the database
//	@Description	Get object classification result from the database
//	@Produce		json
//	@Param			objectID	path	string	true	"object_id"
//	@Tags			diagnose
//	@Success		200	{object}	model.DiagnoseGetReply
//	@Failure		400	{object}	model.DiagnoseGetErr
//	@Failure		404	{object}	model.DiagnoseGetErr
//	@Failure		500	{object}	model.DiagnoseGetErr
//	@Router			/werewolf/diagnose/{objectID} [get]
func (l *ApiClientStruct) GetDiagnoseResultHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err       error
		o, output any
		outErr    model.DiagnoseGetErr
		hasErr    bool
		objectID  uuid.UUID
		h         = &metrica.ServiceApiCallMetricaLabels{
			Model:  "werewolf_diagnose_object_get",
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
	outErr.ImageID = objectID
	o, err = l.GetDiagnoseObject(r.Context(), objectID)
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
