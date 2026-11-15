package database

import (
	"context"

	"github.com/belobragin/werewolf/bus/pkg/filter"
	"github.com/belobragin/werewolf/bus/pkg/model/alter_model"
	"github.com/belobragin/werewolf/bus/pkg/model/diagnose_model"
	"github.com/belobragin/werewolf/bus/pkg/model/list_model"
	"github.com/belobragin/werewolf/bus/pkg/model/picture_model"
	"github.com/google/uuid"
)

// interface to implement database operations. Not database-driver specific
type ProjectDbaseI interface {
	GetVersion() string
	CloseDb() error

	DbPostDiagnoseObject(context.Context, *diagnose_model.DiagnosePostReply) error
	DbGetDiagnoseObject(context.Context, uuid.UUID) (*diagnose_model.DiagnoseGetReply, error)
	DbUploadImage(context.Context, *picture_model.UploadReply) error
	DbDownloadImage(context.Context, uuid.UUID) (*picture_model.DownloadObjectReply, error)
	DbDeleteImage(context.Context, uuid.UUID) error
	DbListCascadeObjects(context.Context, uuid.UUID) ([]picture_model.ObjectKind, error)
	DbGetImageUuidByName(ctx context.Context, imageName string, kind string) (uuid.UUID, error)
	DbGetImageName(context.Context, uuid.UUID) (string, error)
	DbAlterObject(context.Context, *alter_model.AlterPostToDb, uuid.UUID) error
	DbGetAdvDataImageId(context.Context, uuid.UUID) (*alter_model.AlterGetReply, error)
	DbListImage(context.Context, filter.ImageFilter) (*list_model.ListImageReply, error)
}
