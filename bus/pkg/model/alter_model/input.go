package alter_model

import (
	"encoding/json"
	"slices"

	"github.com/belobragin/werewolf/bus/internal/pdata"
)

const (
	Emulator = iota
	BiterateUntarg
	BiterateTarg
	Fgsm
)

var (
	AllowedAlterAlgo = []string{"emulator", "biterate_untargeted", "biterate_targeted", "fgsm"}
	AlgoAlterM       = map[string]int{
		"emulator":            Emulator,
		"biterate_untargeted": BiterateUntarg,
		"biterate_targeted":   BiterateTarg,
		"fgsm":                Fgsm,
	}
)

type InputAlterI interface {
	GetAlgorythm() *string
	GetWithFilter() bool
	GetAlterGoal() string
}

type InputAlter struct {
	Algorythm  *string `json:"algorythm" example:"biterate_targeted" format:"string"`
	WithFilter bool    `json:"with_filter" example:"false" format:"bool"`
	AlterGoal  string  `json:"alter_goal,omitempty" example:"tiger_shark" format:"string"`
}

func (m *InputAlter) GetAlterGoal() string {
	return m.AlterGoal
}

func (m *InputAlter) GetAlgorythm() *string {
	return m.Algorythm
}

func (m *InputAlter) GetWithFilter() bool {
	return m.WithFilter
}

type inputAlterValidate InputAlter

func (m *InputAlter) UnmarshalJSON(b []byte) error {
	var av inputAlterValidate
	if err := json.Unmarshal(b, &av); err != nil {
		return err
	}
	*m = InputAlter(av)
	if s := m.GetAlgorythm(); s == nil {
		return pdata.ErrNilAlgo
	} else if !slices.Contains(AllowedAlterAlgo, *s) {
		return pdata.ErrInvalidAlgo
	} else if *s == "biterate_targeted" && m.AlterGoal == "" {
		return pdata.ErrNilAlterGoal
	}
	return nil
}
