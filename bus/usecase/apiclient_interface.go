package usecase

import (
	"context"
	"net/http"

	"github.com/belobragin/bvv_utils/goutils/abroker"
	sf "github.com/belobragin/bvv_utils/goutils/filter"
	su "github.com/belobragin/bvv_utils/goutils/usecase"
	"github.com/belobragin/werewolf/bus/pkg/database"
	"github.com/belobragin/werewolf/bus/pkg/model/alter_model"
	"github.com/belobragin/werewolf/bus/pkg/model/picture_model"
	"github.com/belobragin/werewolf/bus/pkg/storage"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type ApiClientI interface {
	ApiClientMainI
	ApiClientHelperI
	su.HttpClientI
	FullMetricI
}

// REST API handlers
type ApiClientMainI interface {
	//////////////////////////////////////////////////////////////////
	// picture api
	//////////////////////////////////////////////////////////////////
	// UploadImage uploads picture from local file to storage and put data to database
	// For local storage this method uploads ONLY from $PROJECTFOLDER/$LOCAL_IMAGES_FOLDER to local storage image/ folder
	// For cloud storage this method uploads only from $PROJECTFOLDER/$LOCAL_IMAGES_FOLDER to $AWS_RESOURCE_CFN_STACK_NAME s3 bucket storage/image
	UploadImage(context.Context, picture_model.UploadImageI) (any, error)
	// DownloadImage downloads picture from any storage folder - image/, adversarial/, filter/
	// It must find the file with the given uuid
	// For local storage it downloads file from local storage to the $DOWNLOADS_LOCAL_FOLDER without subfolder but keep file name
	// For cloud storage it downloads object from $AWS_RESOURCE_CFN_STACK_NAME s3 bucket storage to the $DOWNLOADS_CLOUD_FOLDER without subfolder but keep file name
	DownloadImage(context.Context, uuid.UUID) (any, error)
	// DeleteImage deletes uploaded picture or adversarial image or filter image or several if cascade in the storage
	// For local storage it deletes files in the local storage and wipes from the from the database
	// For the cloud storage it deletes objects from the s3 bucket storage and wipes from th cloud database
	DeleteImage(context.Context, uuid.UUID) (any, error)

	//////////////////////////////////////////////////////////////////
	// list api
	//////////////////////////////////////////////////////////////////
	//  ListImage outputs database data for all records that fit the filter
	ListImage(context.Context, sf.ProjectFilterI) (any, error)

	//////////////////////////////////////////////////////////////////
	// diagnose api
	//////////////////////////////////////////////////////////////////
	// PostDiagnoseObject diagnoses picture object by object name and folder
	PostDiagnoseObject(context.Context, string, string) (any, error)
	// PostDiagnoseObjectByID diagnoses picture object by object ID
	PostDiagnoseObjectByID(context.Context, uuid.UUID) (any, error)
	// GetDiagnoseResult outputs diagnose data from the database
	GetDiagnoseObject(context.Context, uuid.UUID) (any, error)
	//
	//////////////////////////////////////////////////////////////////
	// alter api:
	//////////////////////////////////////////////////////////////////
	// PostAlterObject casts adversarial image from the picture object
	PostAlterObject(context.Context, uuid.UUID, alter_model.InputAlterI) (any, error)
	// GetAdversarialObject outputs adversarial data from the database for the input AdversarialID
	GetAdversarialObject(context.Context, uuid.UUID) (any, error)
}

type ApiClientHelperI interface {
	// database
	GetDb() database.ProjectDbaseI
	//
	// storage
	GetStorage() storage.ProjectStorageI
	//
	// broker to publish reply
	GetBroker() abroker.AllBrokerI
	//
	// general
	GetLog() *zap.Logger
	GetDebug() bool
	//
	// request methods
	GetObjectIDFromPath(*http.Request, string) (*string, error)
	//
	// resnet predict api
	GetResnet() su.ResnetPredictI
}

type FullMetricI interface {
	GetApiMetric() su.MetricI
	GetEventMetric() su.MetricI
}
