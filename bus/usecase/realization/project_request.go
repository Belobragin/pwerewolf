package realization

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

func GetObjectIDFromPath(r *http.Request, objName string) (*string, error) {
	id, ok := mux.Vars(r)[objName]
	if !ok || (len(id) == 0) {
		errMessage := errors.New(fmt.Sprintf("%s%s", objName, " is missed"))
		return nil, errMessage
	}
	return &id, nil
}
