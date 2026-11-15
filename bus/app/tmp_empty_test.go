package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/belobragin/werewolf/bus/usecase"
	"go.uber.org/zap"
)

type stubClient struct{ usecase.ApiClientI }

func (stubClient) GetLog() *zap.Logger { return zap.NewNop() }

func TestTmpEmptyBody(t *testing.T) {
	l := &ApiClientStruct{stubClient{}}
	for name, h := range map[string]http.HandlerFunc{
		"upload":   l.UploadImageHandler,
		"diagnose": l.PostDiagnoseObjectHandler,
	} {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodPost, "/x?image_name=dig.png", strings.NewReader("")))
		t.Logf("%s: %d %s", name, rec.Code, rec.Body.String())
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: code %d", name, rec.Code)
		}
	}
}
