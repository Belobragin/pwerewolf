package filter

import (
	"errors"
	"net/url"
	"slices"

	sf "github.com/belobragin/bvv_utils/goutils/filter"
	"github.com/belobragin/werewolf/bus/pkg/model/alter_model"
	"github.com/fatih/structs"
	"github.com/google/uuid"
	"github.com/gorilla/schema"
)

var (
	ErrIncorrectFilterQuery = errors.New("incorrect filter query")
	ErrInvalidFilter        = errors.New("Invalid or un-intialized filter")
)

type ImageFilter struct {
	ImageID         *uuid.UUID `json:"image_uuid,omitempty" schema:"image_uuid,omitempty"`
	AdversarialID   *uuid.UUID `json:"adversarial_object_uuid,omitempty" schema:"adversarial_id,omitempty"`
	FilterID        *uuid.UUID `json:"filter_object_uuid,omitempty" schema:"filter_id, omitempty"`
	AlterGoal       *string    `json:"alter_goal,omitempty" schema:"alter_goal,omitempty"`
	AdversarialName *string    `json:"adversarial_name,omitempty" schema:"adversarial_name,omitempty"`
	FilterName      *string    `json:"filter_object_name,omitempty" schema:"filter_object_name,omitempty"`
	ImageName       *string    `json:"image_name,omitempty" schema:"image_name,omitempty"`
	Algorythm       *string    `json:"algorythm,omitempty" schema:"algorythm,omitempty"`
}

func NewImageFilter() (sf.ProjectFilterI, error) {
	var f = new(ImageFilter)

	return f, nil
}
func (f *ImageFilter) ParseQueries(u url.Values) error {
	var decoder = schema.NewDecoder()
	if f == nil {
		return ErrInvalidFilter
	}
	err := decoder.Decode(f, u)
	if err != nil {
		return ErrIncorrectFilterQuery
	}
	return nil
}

func (f *ImageFilter) DistillateQuery() (map[string]interface{}, error) {
	if f.Algorythm != nil && !slices.Contains(alter_model.AllowedAlterAlgo, *f.Algorythm) {
		f.Algorythm = nil
	}
	m := make(map[string]interface{}, 5)
	m = structs.Map(f)

	return m, nil
}
