package alter_model

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/belobragin/werewolf/bus/pkg/model/diagnose_model"
	"github.com/google/uuid"
)

var (
	ErrMLFailed       = errors.New("ML service reported a failure")
	ErrInvalidMLReply = errors.New("ML service reply is invalid")
)

// AlterCheckUniqueImageAlgo is to check
type AlterCheckUniqueImageAlgo struct {
	ImageID      uuid.UUID `json:"image_uuid" example:"550e8400-e29b-41d4-a716-446655440000" format:"uuid"`
	Algorythm    string    `json:"algorythm" example:"biterate_targeted" format:"string"`
	WipeExistent bool      `json:"wipe_exist" example:"true" format:"bool"`
}

// MLMessageRequest is to send it as a nats message body
type MLMessageRequest struct {
	ImageID    uuid.UUID `json:"order_id" example:"550e8400-e29b-41d4-a716-446655440000" format:"string"`
	Algorythm  string    `json:"algorythm" example:"biterate_targeted" format:"string"`
	ImageName  string    `json:"input_image_name" example:"hog-test.jpg" format:"string"`
	AlterGoal  string    `json:"alter_goal" example:"tiger_shark" format:"string"`
	SaveFilter string    `json:"save_filter" example:"1" format:"string"`
}

// AlterPostReply is to output from http logic adapter
type AlterPostReply struct {
	MLMessageRequest `json:"ml_request"`
	CreatedAt        string `json:"created_at" example:"2023-08-28T14:11:51.213+0300" format:"string"`
	Code             int    `json:"code" example:"200" format:"int64"`
}

// LooseBool reads true/false as well as "1"/"0"/"true"/"false": the ML service echoes the request's save_filter string as success_filter.
type LooseBool bool

func (b *LooseBool) UnmarshalJSON(data []byte) error {
	switch strings.Trim(strings.ToLower(string(data)), `"`) {
	case "true", "1":
		*b = true
	case "false", "0", "", "null":
		*b = false
	default:
		return fmt.Errorf("invalid boolean value %s", data)
	}
	return nil
}

// type MLMessageReply is to obtain data from ML service
// example of the ML reply:
// {"order_id": "5678234", "success_adversarial": true, "success_filter": false, "adversarial_name": "/aker-data/adversarial/hog-test_adversarial_biterate_targeted.jpg", "timestamp": "2026-10-20T14:03:45.270005", "error": "", "algorythm": "biterate_targeted", "alter_goal": "tiger_shark", "predictions": {"n0": ["n01695060", "Komodo_dragon", "48", "52.38"], "n1": ["n01806143", "peacock", "84", "7.82"], "n2": ["n02454379", "armadillo", "363", "7.71"]}
// filter_name is sent only when success_filter is set
type MLMessageReply struct {
	ImageID            string              `json:"order_id" example:"550e8400-e29b-41d4-a716-446655440000" format:"string"`
	SuccessAdversarial bool                `json:"success_adversarial" example:"true" format:"bool"`
	SuccessFilter      LooseBool           `json:"success_filter" example:"false" format:"bool"`
	AdversarialName    string              `json:"adversarial_name" example:"/aker-data/adversarial/hog-test_adversarial_biterate_targeted.jpg" format:"string"`
	FilterName         string              `json:"filter_name,omitempty" example:"/aker-data/filter/hog-test_filter_biterate_targeted.jpg" format:"string"`
	Timestamp          string              `json:"timestamp" example:"2026-10-20T14:03:45.270005" format:"string"`
	Error              string              `json:"error" example:"" format:"string"`
	Algorythm          string              `json:"algorythm" example:"biterate_targeted" format:"string"`
	AlterGoal          string              `json:"alter_goal" example:"tiger_shark" format:"string"`
	Predictions        map[string][]string `json:"predictions"`
}

// AlterPostToDb is the ML reply prepared for the database: new adversarial object, optional filter, and the diagnose result.
// All data belong to ImageID ("order_id" of the ML reply).
type AlterPostToDb struct {
	ImageID             uuid.UUID                                        `json:"image_uuid" example:"550e8400-e29b-41d4-a716-446655440000" format:"uuid"`
	AdversarialObjectID uuid.UUID                                        `json:"adversarial_object_id" example:"550e8400-e29b-41d4-a716-446655440111" format:"uuid"`
	AdversarialName     string                                           `json:"adversarial_object_name" example:"hog-test_adversarial_biterate_targeted.jpg" format:"string"`
	Algorythm           string                                           `json:"algorythm" example:"biterate_targeted" format:"string"`
	AlterGoal           string                                           `json:"alter_goal,omitempty" example:"tiger_shark" format:"string"`
	FilterObjectID      uuid.UUID                                        `json:"filter_object_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440222" format:"uuid"`
	FilterName          string                                           `json:"filter_name,omitempty" example:"hog-test_filter_biterate_targeted.jpg" format:"string"`
	Predict             []map[string]diagnose_model.ResnetPredictionData `json:"predict,omitempty"`
	CreatedAt           string                                           `json:"created_at" example:"2023-08-28T14:11:51.213+0300" format:"string"`
}

// HasFilter reports whether a filter object comes with the adversarial one.
func (a *AlterPostToDb) HasFilter() bool {
	return a.FilterObjectID != uuid.Nil
}

// ToAlterPostToDb makes new uuids for the adversarial object and, if the ML service made one, for the filter.
// Names are cut to the file name, as the storage keeps files by name only.
func (m *MLMessageReply) ToAlterPostToDb() (*AlterPostToDb, error) {
	if !m.SuccessAdversarial || m.Error != "" {
		return nil, fmt.Errorf("%w: %s", ErrMLFailed, m.Error)
	}
	imageID, err := uuid.Parse(m.ImageID)
	if err != nil {
		return nil, fmt.Errorf("%w: order_id: %v", ErrInvalidMLReply, err)
	}
	if m.AdversarialName == "" || m.Algorythm == "" {
		return nil, fmt.Errorf("%w: adversarial_name and algorythm are required", ErrInvalidMLReply)
	}

	out := &AlterPostToDb{
		ImageID:             imageID,
		AdversarialObjectID: uuid.New(),
		AdversarialName:     filepath.Base(m.AdversarialName),
		Algorythm:           m.Algorythm,
		AlterGoal:           m.AlterGoal,
		CreatedAt:           time.Now().Format(time.RFC3339Nano),
	}
	if m.SuccessFilter {
		if m.FilterName == "" {
			return nil, fmt.Errorf("%w: success_filter is set, but filter_name is empty", ErrInvalidMLReply)
		}
		out.FilterObjectID = uuid.New()
		out.FilterName = filepath.Base(m.FilterName)
	}
	if len(m.Predictions) > 0 {
		r, err := (&diagnose_model.ResnetServerReply{Predictions: m.Predictions}).ToResnetReply()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidMLReply, err)
		}
		out.Predict = r.Predict
	}
	return out, nil
}

type AlterPostErr struct {
	ObjectID   uuid.UUID `json:"object_id" example:"550e8400-e29b-41d4-a716-446655440000" format:"uuid"`
	Code       int       `json:"code" example:"400" format:"int64"`
	CreatedAt  string    `json:"created_at" example:"2023-08-28T14:11:51.213+0300" format:"string"`
	ErrMessage string    `json:"err_message" example:"not found" format:"string"`
}

func (w *AlterPostErr) Error() string {
	return w.ErrMessage
}

type AlterGetOneReply struct {
	ImageID               uuid.UUID `json:"image_uuid" example:"550e8400-e29b-41d4-a716-446655440000" format:"uuid"`
	AdversarialObjectID   uuid.UUID `json:"adversarial_object_id" example:"550e8400-e29b-41d4-a716-446655440000" format:"uuid"`
	AdversarialObjectName string    `json:"adversarial_object_name" example:"/adversarial_image/dog-115_adv.img" format:"string"`
	FilterObjectID        uuid.UUID `json:"filter_object_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000" format:"uuid"`
	FilterName            string    `json:"filter_name,omitempty" example:"/filter_image/dog-115_filter.img" format:"string"`
	Algorythm             string    `json:"algorythm" example:"biterate_targeted" format:"string"`
	AlterGoal             string    `json:"alter_goal,omitempty" example:"tiger_shark" format:"string"`
	CreatedAt             string    `json:"created_at" example:"2023-08-28T14:11:51.213+0300" format:"string"`
}

type AlterGetReply struct {
	AltergetAllreply []AlterGetOneReply `json:"alter_all_reply"`
	Code             int                `json:"code" example:"400" format:"int64"`
}

type AlterGetErr AlterPostErr
