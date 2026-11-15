package adapter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/belobragin/bvv_utils/goutils/mistake"
	"github.com/belobragin/werewolf/bus/pkg/model/diagnose_model"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func (l *ApiClientRealization) PostDiagnoseObject(
	ctx context.Context, imageName, imageFolder string,
) (any, error) {
	var (
		output       diagnose_model.DiagnosePostReply
		resNetOutput diagnose_model.ResnetServerReply
	)

	qq := make(url.Values)
	qq.Set("image_name", imageName)
	qq.Set("folder", imageFolder)

	req, err := l.NewRequestWithContext(ctx, "POST", l.GetResnet().GetPredictApiUrl(), nil)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = qq.Encode()
	resp, err := l.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = mistake.ErrInvalidStatusCode
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &resNetOutput)
	if err != nil {
		return nil, err
	}
	resnetReply, err := resNetOutput.ToResnetReply()
	if err != nil {
		return nil, err
	}

	db := l.GetDb()
	output.ImageID, err = db.DbGetImageUuidByName(ctx, imageName, imageFolder)
	if err != nil {
		return nil, err
	}
	output.ResnetReply = resnetReply
	output.CreatedAt = time.Now().Format(time.RFC3339Nano)

	err = db.DbPostDiagnoseObject(ctx, &output)
	if err != nil {
		return nil, err
	}
	if log := l.GetLog(); log != nil {
		log.Debug("image is diagnosed",
			zap.String("image_name", imageName),
			zap.String("folder", imageFolder),
			zap.String("image_uuid", output.ImageID.String()),
			zap.Any("predictions", resnetReply.Predict),
		)
	}

	output.Code = http.StatusCreated

	return any(output), nil
}

func (l *ApiClientRealization) PostDiagnoseObjectByID(
	ctx context.Context, objectId uuid.UUID,
) (any, error) {
	imageName, err := l.GetDb().DbGetImageName(ctx, objectId)
	if err != nil {
		return nil, err
	}

	return l.PostDiagnoseObject(ctx, imageName, "image")
}

func (l *ApiClientRealization) GetDiagnoseObject(
	ctx context.Context, objectId uuid.UUID,
) (any, error) {
	db := l.GetDb()
	output, err := db.DbGetDiagnoseObject(ctx, objectId)
	if err != nil {
		return nil, err
	}

	output.Code = http.StatusOK

	return any(*output), nil
}
