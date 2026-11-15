package app

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	m "github.com/belobragin/bvv_utils/goutils/middleware"
	metrica "github.com/belobragin/bvv_utils/goutils/usecase"
	"github.com/belobragin/werewolf/bus/internal/pdata"
	model "github.com/belobragin/werewolf/bus/pkg/model/picture_model"
	"github.com/google/uuid"
)

// UploadImageHandler 	godoc
//
//	@Summary		Upload image to the storage and make database record
//	@Description	Upload image file for next adversarial transformation or for diagnosing to the storage and make database record
//	@Accept			x-www-form-urlencoded
//	@Produce		json
//	@Param			UploadImage	body	model.UploadImage	true	"input upload image data"
//	@Tags			picture
//	@Success		200	{object}	model.UploadReply
//	@Failure		400	{object}	model.UploadErr
//	@Failure		404	{object}	model.UploadErr
//	@Failure		406	{object}	model.UploadErr
//	@Failure		409	{object}	model.UploadErr
//	@Failure		500	{object}	model.UploadErr
//	@Router			/werewolf/picture/upload [post]
func (l *ApiClientStruct) UploadImageHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err       error
		body      []byte
		req       model.UploadImage
		outErr    model.UploadErr
		o, output any
		hasErr    bool
		h         = &metrica.ServiceApiCallMetricaLabels{
			Model:  "werewolf_upload_image_post",
			Method: r.Method,
		}
	)
	zapstruct := l.GetLog()
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
	outErr.FilePath = req.GetImagePath()
	o, err = l.UploadImage(r.Context(), &req)
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

// DownloadObjectHandler 	godoc
//
//	@Summary		Download object to local file in $PROJECTFOLDER/download folder
//	@Description	Download initial, adversarial or filter picture to local file in $PROJECTFOLDER/download folder
//	@Accept			x-www-form-urlencoded
//	@Produce		json
//	@Param			objectID	path	string	true	"object_id"
//	@Tags			picture
//	@Success		201	{object}	model.DownloadObjectReply
//	@Failure		400	{object}	model.DownloadObjectErr
//	@Failure		404	{object}	model.DownloadObjectErr
//	@Failure		406	{object}	model.DownloadObjectErr
//	@Failure		500	{object}	model.DownloadObjectErr
//	@Router			/werewolf/picture/download/{objectID} [post]
func (l *ApiClientStruct) DownloadObjectHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err       error
		outErr    model.DownloadObjectErr
		o, output any
		hasErr    bool
		objectID  uuid.UUID
		h         = &metrica.ServiceApiCallMetricaLabels{
			Model:  "werewolf_download_object_post",
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
	o, err = l.DownloadImage(r.Context(), objectID)
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

// DeleteObjectHandler 	godoc
//
//	@Summary		Delete object in storage and wipe out data from database
//	@Description	Delete object (initial/adversarial image or filter) in storage and wipe out data from database
//	@Accept			x-www-form-urlencoded
//	@Produce		json
//	@Param			objectID	path	string	true	"object_id"
//	@Tags			picture
//	@Success		200	{object}	model.DeleteObjectReply
//	@Failure		400	{object}	model.DeleteObjectErr
//	@Failure		404	{object}	model.DeleteObjectErr
//	@Failure		409	{object}	model.DeleteObjectErr
//	@Failure		500	{object}	model.DeleteObjectErr
//	@Router			/werewolf/picture/{objectID} [delete]
func (l *ApiClientStruct) DeleteObjectHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err       error
		outErr    model.DeleteObjectErr
		o, output any
		hasErr    bool
		objectID  uuid.UUID
		h         = &metrica.ServiceApiCallMetricaLabels{
			Model:  "werewolf_delete_object_delete",
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
	o, err = l.DeleteImage(r.Context(), objectID)
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
