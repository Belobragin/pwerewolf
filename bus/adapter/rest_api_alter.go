package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	sf "github.com/belobragin/bvv_utils/goutils/filter"
	"github.com/belobragin/werewolf/bus/internal/pdata"
	model "github.com/belobragin/werewolf/bus/pkg/model/alter_model"
	"github.com/google/uuid"
)

func (l *ApiClientRealization) PostAlterObject(
	ctx context.Context, objectId uuid.UUID, input model.InputAlterI,
) (any, error) {
	algorythm := input.GetAlgorythm()
	if algorythm == nil || !slices.Contains(model.AllowedAlterAlgo, *algorythm) {
		return nil, pdata.ErrInvalidAlgo
	}

	imageName, err := l.GetDb().DbGetImageName(ctx, objectId)
	if err != nil {
		return nil, err
	}

	request := model.MLMessageRequest{
		ImageID:   objectId,
		Algorythm: *algorythm,
		ImageName: imageName,
		AlterGoal: input.GetAlterGoal(),
	}
	if input.GetWithFilter() {
		// aker takes any non-empty string as true, so false must stay empty
		request.SaveFilter = "1"
	}
	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", pdata.ErrInvalidAlterMessage, err)
	}
	if err = l.Broker.PublishEvent(data); err != nil {
		return nil, fmt.Errorf("%w: %v", pdata.ErrPublish, err)
	}

	return any(model.AlterPostReply{
		MLMessageRequest: request,
		CreatedAt:        time.Now().Format(time.RFC3339Nano),
		Code:             http.StatusOK,
	}), nil
}

func (l *ApiClientRealization) ListAlterObject(ctx context.Context, f sf.ProjectFilterI) (any, error) {
	m, err := f.DistillateQuery()
	if err != nil {
		return nil, nil
	}
	// TODO: write method
	fmt.Printf("%+v/n", m)

	return nil, nil
}

func (l *ApiClientRealization) GetAdversarialObject(ctx context.Context, objectID uuid.UUID) (any, error) {
	output, err := l.GetDb().DbGetAdvDataImageId(ctx, objectID)
	if err != nil {
		return nil, err
	}

	output.Code = http.StatusOK

	return any(*output), nil
}
