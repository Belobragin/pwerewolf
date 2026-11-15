package port

import (
	"net/http"

	"github.com/belobragin/werewolf/bus/app"
	"github.com/belobragin/werewolf/bus/usecase"
	"github.com/gorilla/mux"
)

const (
	apiPrefix           = "/api/v1"
	WerewolfServiceName = "werewolf"
	pictureGroup        = "picture"
	diagnoseGroup       = "diagnose"
	alterGroup          = "alter"
	listGroup           = "list"
	objectGroup         = "object"
	tokenGroup          = "token"
	docsGroup           = "docs"
)

var (
	ProjectRouter = mux.NewRouter().StrictSlash(true)

	RouteFunction = func(farg func(w http.ResponseWriter, r *http.Request), route, method, apiName string) *mux.Route {
		return ProjectRouter.Path(route).
			HandlerFunc(farg).
			Methods(method).
			Name(apiName)
	}
)

var (
	WfPostUploadImageApiName        = WerewolfServiceName + "_post_upload_picture"
	WfGetListImageApiName           = WerewolfServiceName + "_get_list_image"
	WfPostDownloadObjectApiName     = WerewolfServiceName + "_post_download_picture"
	WfDeleteObjectApiName           = WerewolfServiceName + "_delete_picture"
	WfPostAlterObjectApiName        = WerewolfServiceName + "_post_alter_image"
	WfGetAdversarialObjectApiName   = WerewolfServiceName + "_get_alter_image"
	WfPostDiagnoseObjectApiName     = WerewolfServiceName + "_post_diagnose_picture"
	WfPostDiagnoseObjectByIDApiName = WerewolfServiceName + "_post_diagnose_picture_byid"
	WfGetDiagnoseObjectApiName      = WerewolfServiceName + "_get_diagnose_picture"

	WfDocsRouteApiName = WerewolfServiceName + "_docs_swagger"

	WfPostUploadImageRoute        = apiPrefix + "/" + WerewolfServiceName + "/" + pictureGroup + "/upload"
	WfGetListImageRoute           = apiPrefix + "/" + WerewolfServiceName + "/" + listGroup
	WfPostDownloadObjectRoute     = apiPrefix + "/" + WerewolfServiceName + "/" + pictureGroup + "/download/{objectID}"
	WfDeleteObjectRoute           = apiPrefix + "/" + WerewolfServiceName + "/" + pictureGroup + "/{objectID}"
	WfPostDiagnoseObjectRoute     = apiPrefix + "/" + WerewolfServiceName + "/" + diagnoseGroup
	WfPostDiagnoseObjectByIDRoute = apiPrefix + "/" + WerewolfServiceName + "/" + diagnoseGroup + "/{objectID}"
	WfGetDiagnoseObjectRoute      = apiPrefix + "/" + WerewolfServiceName + "/" + diagnoseGroup + "/{objectID}"
	WfPostAlterObjectRoute        = apiPrefix + "/" + WerewolfServiceName + "/" + alterGroup + "/{objectID}"
	WfGetAdversarialObjectRoute   = apiPrefix + "/" + WerewolfServiceName + "/" + alterGroup + "/{objectID}"
	WfListAlterResultRoute        = apiPrefix + "/" + WerewolfServiceName + "/" + alterGroup + "/list"

	WfDocsRoute = apiPrefix + "/" + WerewolfServiceName + "/" + docsGroup + "/swagger"
)

func AddNewWerewolfRoutes(apiClient usecase.ApiClientI) {

	apiHandler := &app.ApiClientStruct{apiClient}

	// picture api:
	//
	RouteFunction(apiHandler.UploadImageHandler, WfPostUploadImageRoute, http.MethodPost, WfPostUploadImageApiName)
	RouteFunction(apiHandler.ListImageHandler, WfGetListImageRoute, http.MethodPost, WfGetListImageApiName)
	RouteFunction(apiHandler.DownloadObjectHandler, WfPostDownloadObjectRoute, http.MethodPost, WfPostDownloadObjectApiName)
	RouteFunction(apiHandler.DeleteObjectHandler, WfDeleteObjectRoute, http.MethodDelete, WfDeleteObjectApiName)

	// diagnose api:
	//
	RouteFunction(apiHandler.PostDiagnoseObjectHandler, WfPostDiagnoseObjectRoute, http.MethodPost, WfPostDiagnoseObjectApiName)
	RouteFunction(apiHandler.PostDiagnoseObjectByIDHandler, WfPostDiagnoseObjectByIDRoute, http.MethodPost, WfPostDiagnoseObjectByIDApiName)
	RouteFunction(apiHandler.GetDiagnoseResultHandler, WfGetDiagnoseObjectRoute, http.MethodGet, WfGetDiagnoseObjectApiName)

	// alter object:
	//
	RouteFunction(apiHandler.AlterObjectHandler, WfPostAlterObjectRoute, http.MethodPost, WfPostAlterObjectApiName)
	RouteFunction(apiHandler.GetAdversarialResultbyImageIDHandler, WfGetAdversarialObjectRoute, http.MethodGet, WfGetAdversarialObjectApiName)

	// list images
	//
	RouteFunction(apiHandler.ListImageHandler, WfGetListImageRoute, http.MethodGet, WfGetListImageApiName)

	if apiHandler.GetDebug() {
		RouteFunction(apiHandler.SwaggerDocsHandler, WfDocsRoute, http.MethodGet, WfDocsRouteApiName)
	}
}
