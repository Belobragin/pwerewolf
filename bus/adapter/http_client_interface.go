package adapter

import (
	"net/http"
)

type HttpClientI interface {
	Do(r *http.Request) (*http.Response, error)
}
