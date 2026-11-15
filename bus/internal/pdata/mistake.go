package pdata

import (
	"errors"
	"net/http"
)

var (
	ErrNilAlgo      = errors.New("not specified algorytm")
	ErrInvalidAlgo  = errors.New("invalid alter algorytm")
	ErrNilAlterGoal = errors.New("alter_goal is required for the targeted algorytm")
)

// database errors
var (
	ErrNoObjectId        = errors.New("no database record with the given object_uuid")
	ErrNoResultInDb      = errors.New("database record for the given object_uuid does not exist")
	ErrExistObjectID     = errors.New("object uuid already exists")
	ErrExistObjectName   = errors.New("object name already exists")
	ErrRepeatImageAlgo   = errors.New("adversarial with pair image_uuid - algorythm exist")
	ErrNoObjectName      = errors.New("no database record with the given object name")
	ErrInvalidObjectKind = errors.New("invalid object kind, must be image, adversarial or filter")
	ErrInvalidListFilter = errors.New("filter is not an image filter")
)

// image file errors
var (
	ErrInvalidFilePath = errors.New("file path must be relative to the project folder and inside the local images folder")
	ErrNoSourceFile    = errors.New("source image file does not exist")
	ErrExistImageFile  = errors.New("image file already exists in the storage")
	ErrImageTooLarge   = errors.New("image file is too large")
	ErrInvalidImage    = errors.New("file is not a supported image (jpeg, png or gif)")
	ErrNoStorageObject = errors.New("object file does not exist in the storage")
	ErrDeleteFile      = errors.New("object file can not be deleted from the storage")
	ErrDeleteRecord    = errors.New("object file is deleted, but the database record is not")
)

// ML service (aker) errors
var (
	ErrInvalidAlterMessage = errors.New("alter message for the ML service is invalid")
	ErrPublish             = errors.New("alter message is not delivered to the ML service")
)

func GetCodeFromErr(e error) int {
	if errors.Is(e, ErrNoObjectId) || errors.Is(e, ErrNoResultInDb) || errors.Is(e, ErrNoObjectName) ||
		errors.Is(e, ErrNoSourceFile) {
		return http.StatusNotFound
	} else if errors.Is(e, ErrInvalidObjectKind) || errors.Is(e, ErrInvalidFilePath) || errors.Is(e, ErrInvalidImage) ||
		errors.Is(e, ErrInvalidAlterMessage) || errors.Is(e, ErrInvalidListFilter) {
		return http.StatusBadRequest
	} else if errors.Is(e, ErrPublish) {
		return http.StatusBadGateway
	} else if errors.Is(e, ErrImageTooLarge) || errors.Is(e, ErrNoStorageObject) || errors.Is(e, ErrDeleteRecord) {
		return http.StatusNotAcceptable
	} else if errors.Is(e, ErrExistObjectID) || errors.Is(e, ErrExistObjectName) ||
		errors.Is(e, ErrRepeatImageAlgo) || errors.Is(e, ErrExistImageFile) || errors.Is(e, ErrDeleteFile) {
		return http.StatusConflict
	}
	return http.StatusInternalServerError

}
