package list_model

import (
	"github.com/google/uuid"
)

type OneImageReply struct {
	ImageID         *uuid.UUID `json:"image_uuid,omitempty" schema:"image_uuid,omitempty"`
	AdversarialID   *uuid.UUID `json:"adversarial_object_uuid,omitempty" schema:"adversarial_id,omitempty"`
	FilterID        *uuid.UUID `json:"filter_object_uuid,omitempty" schema:"filter_id, omitempty"`
	AlterGoal       *string    `json:"alter_goal,omitempty" schema:"alter_goal,omitempty"`
	AdversarialName *string    `json:"adversarial_name,omitempty" schema:"adversarial_name,omitempty"`
	FilterName      *string    `json:"filter_object_name,omitempty" schema:"filter_object_name,omitempty"`
	ImageName       *string    `json:"image_name,omitempty" schema:"image_name,omitempty"`
	Algorythm       *string    `json:"algorythm,omitempty" schema:"algorythm,omitempty"`
}

type ListImageReply struct {
	AllImage []OneImageReply `json:"all_image"`
	Code     int             `json:"code" example:"400" format:"int64"`
}

type ListImageErr struct {
	Filter     string `json:"picture_filter" format:"string"`
	Code       int    `json:"code" example:"400" format:"int64"`
	ErrMessage string `json:"err_message" example:"not found" format:"string"`
	CreatedAt  string `json:"created_at" example:"2023-08-28T14:11:51.213+0300" format:"string"`
}
