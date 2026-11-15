package picture_model

import (
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"
)

const maxImageDescriptionLen = 1800

type UploadImageI interface {
	GetImagePath() string
	GetFileDescription() string
}

type UploadImage struct {
	FilePath        string `json:"file_path" example:"test-image/dog-115.img" format:"string"`
	FileDescription string `json:"file_description" example:"test image with the dog" format:"string"`
}

func (w *UploadImage) GetImagePath() string {
	return w.FilePath
}
func (w *UploadImage) GetFileDescription() string {
	return w.FileDescription
}

type uploadImageValidate UploadImage

func (m *UploadImage) UnmarshalJSON(b []byte) error {
	var av uploadImageValidate
	if err := json.Unmarshal(b, &av); err != nil {
		return err
	}
	*m = UploadImage(av)
	// existence is checked in the usecase relative to $PROJECTFOLDER, not to the process cwd.
	if m.GetImagePath() == "" {
		return errors.New("file_path is required")
	}
	if utf8.RuneCountInString(m.GetFileDescription()) > maxImageDescriptionLen {
		return fmt.Errorf("file_description is longer than %d characters", maxImageDescriptionLen)
	}
	return nil
}
