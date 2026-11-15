package picture_model

import (
	"github.com/belobragin/werewolf/bus/pkg/storage"
	"github.com/google/uuid"
)

type UploadReply struct {
	ImageID          uuid.UUID `json:"image_uuid" example:"550e8400-e29b-41d4-a716-446655440000" format:"uuid"`
	ImageName        string    `json:"image_name" example:"dog-115.img" format:"string"`
	ImageDescription string    `json:"image_description,omitempty" example:"test image with the dog" format:"string"`
	MimeType         string    `json:"mime_type" example:"image/jpeg" format:"string"`
	ImageSize        string    `json:"image_size" example:"64Kb" format:"string"`
	ImageHeight      int       `json:"image_height" example:"600" format:"int64"`
	ImageWidth       int       `json:"image_width" example:"400" format:"int64"`
	CreatedAt        string    `json:"created_at" example:"2023-08-28T14:11:51.213+0300" format:"string"`
}

type UploadErr struct {
	FilePath   string `json:"file_path" example:"/home/user/images/werewolf/dog-115.img" format:"string"`
	Code       int    `json:"code" example:"400" format:"int64"`
	ErrMessage string `json:"err_message" example:"not found" format:"string"`
	CreatedAt  string `json:"created_at" example:"2023-08-28T14:11:51.213+0300" format:"string"`
}

type DownloadObjectReply struct {
	ObjectID      uuid.UUID `json:"object_id" example:"550e8400-e29b-41d4-a716-446655440000" format:"string"`
	IfImage       bool      `json:"if_image,omitempty" example:"true" format:"bool"`
	IfAdversarial bool      `json:"if_adversarial,omitempty" example:"true" format:"bool"`
	IfFilter      bool      `json:"if_filter,omitempty" example:"true" format:"bool"`
	ObjectName    string    `json:"object_name" example:"dog-115_filter.img" format:"string"`
	CreatedAt     string    `json:"created_at" example:"2026-08-28T14:11:51.213+0300" format:"string"`
	Code          int       `json:"code" example:"200" format:"int64"`
}

type DownloadObjectErr struct {
	ObjectID   uuid.UUID `json:"object_id" example:"550e8400-e29b-41d4-a716-446655440000" format:"string"`
	Code       int       `json:"code" example:"400" format:"int64"`
	ErrMessage string    `json:"err_message" example:"not found" format:"string"`
	CreatedAt  string    `json:"created_at" example:"2023-08-28T14:11:51.213+0300" format:"string"`
}

type ObjectKind struct {
	ID        uuid.UUID          `json:"object_id" example:"550e8400-e29b-41d4-a716-446655440111" format:"string"`
	Name      string             `json:"object_name" example:"hog-test_adversarial_biterate_targeted.jpg" format:"string"`
	Kind      storage.ObjectKind `json:"object_kind" example:"adversarial" format:"string"`
	CreatedAt string             `json:"created_at" example:"2026-08-28T14:11:51.213+0300" format:"string"`
}

type DeleteObjectReply struct {
	AllObject []ObjectKind `json:"all_object"`
	DeletedAt string       `json:"deleted_at" example:"2026-08-29T14:11:51.213+0300" format:"string"`
	Code      int          `json:"code" example:"200" format:"int64"`
}

type DeleteObjectErr DownloadObjectErr
