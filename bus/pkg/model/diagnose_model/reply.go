package diagnose_model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrEmptyPrediction   = errors.New("resnet reply has no predictions")
	ErrInvalidPrediction = errors.New("resnet reply has invalid prediction")
)

type ResnetPredictionData struct {
	ClassLabel  string  `json:"class_label" example:"n02396427" format:"string"`
	ClassName   string  `json:"class_name" example:"wild_boar" format:"string"`
	ClassID     int     `json:"class_id" example:"342" format:"int64"`
	Probability float32 `json:"probability" example:"82.28" format:"float32"`
}

// ResnetServerReply is the raw reply of the resnet predict server. Each prediction is
// [class_label, class_name, class_id, probability] under the key "n<rank>".
type ResnetServerReply struct {
	Image       string              `json:"image"`
	Folder      string              `json:"folder"`
	Predictions map[string][]string `json:"predictions"`
}

// ToResnetReply returns predictions ordered by rank: Predict[i] holds the key "n<i>".
func (s *ResnetServerReply) ToResnetReply() (ResnetReply, error) {
	if len(s.Predictions) == 0 {
		return ResnetReply{}, ErrEmptyPrediction
	}
	reply := ResnetReply{
		ImageName: s.Image,
		Predict:   make([]map[string]ResnetPredictionData, len(s.Predictions)),
	}
	for key, v := range s.Predictions {
		rank, err := strconv.Atoi(strings.TrimPrefix(key, "n"))
		if err != nil || rank < 0 || rank >= len(s.Predictions) || reply.Predict[rank] != nil {
			return ResnetReply{}, fmt.Errorf("%w: key %q", ErrInvalidPrediction, key)
		}
		if len(v) != 4 {
			return ResnetReply{}, fmt.Errorf("%w: key %q has %d fields, want 4", ErrInvalidPrediction, key, len(v))
		}
		classID, err := strconv.Atoi(v[2])
		if err != nil {
			return ResnetReply{}, fmt.Errorf("%w: key %q class id: %v", ErrInvalidPrediction, key, err)
		}
		probability, err := strconv.ParseFloat(v[3], 32)
		if err != nil {
			return ResnetReply{}, fmt.Errorf("%w: key %q probability: %v", ErrInvalidPrediction, key, err)
		}
		reply.Predict[rank] = map[string]ResnetPredictionData{
			key: {ClassLabel: v[0], ClassName: v[1], ClassID: classID, Probability: float32(probability)},
		}
	}
	return reply, nil
}

type ResnetReply struct {
	ImageName string                            `json:"image_name" example:"hog.jpg" format:"string"`
	Predict   []map[string]ResnetPredictionData `json:"predict"`
}

// TODO-BV
func (r *ResnetReply) Validate() error {
	return nil
}

type DbResultReply struct {
	Predict   []map[string]ResnetPredictionData `json:"prediction"`
	CreatedAt string                            `json:"created_at" example:"2026-08-28T14:11:51.213+0300" format:"string"`
	UpdatedAt string                            `json:"updated_at" example:"2026-08-28T14:13:51.213+0300" format:"string"`
}

type DiagnosePostReply struct {
	ResnetReply ResnetReply `json:"resnet_reply"`
	ImageID     uuid.UUID   `json:"image_uuid" example:"550e8400-e29b-41d4-a716-446655440000" format:"uuid"`
	CreatedAt   string      `json:"created_at" example:"2026-08-28T14:11:51.213+0300" format:"string"`
	Code        int         `json:"code" example:"200" format:"int64"`
}

type DiagnosePostErr struct {
	ImageID    uuid.UUID `json:"image_uuid" example:"550e8400-e29b-41d4-a716-446655440000" format:"uuid"`
	Code       int       `json:"code" example:"400" format:"int64"`
	ErrMessage string    `json:"err_message" example:"not found" format:"string"`
	CreatedAt  string    `json:"created_at" example:"2023-08-28T14:11:51.213+0300" format:"string"`
}

type DiagnoseGetReply struct {
	DbResultReply DbResultReply `json:"result_reply"`
	ImageName     string        `json:"image_name" example:"hog.jpg" format:"string"`
	ImageID       uuid.UUID     `json:"image_uuid" example:"550e8400-e29b-41d4-a716-446655440000" format:"uuid"`
	Code          int           `json:"code" example:"200" format:"int64"`
}

type DiagnoseGetErr DiagnosePostErr
