package middleware

import (
	"fmt"
	"net/http"

	"github.com/libtnb/chix/v2"
)

func Abort(w http.ResponseWriter, code int, format string, args ...any) {
	render := chix.NewRender(w)
	defer render.Release()
	render.Header(chix.HeaderContentType, chix.MIMEApplicationJSONCharsetUTF8) // must before Status()
	render.Status(code)
	if len(args) > 0 {
		format = fmt.Sprintf(format, args...)
	}
	render.JSON(chix.M{
		"msg": format,
	})
}
