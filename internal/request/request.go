package request

import (
	"net/http"
)

type WithPrepare interface {
	Prepare(r *http.Request) error
}
