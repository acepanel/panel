package service

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/libtnb/chix/v2"
	"github.com/libtnb/validator"

	"github.com/acepanel/panel/v3/internal/request"
)

// SuccessResponse 通用成功响应
type SuccessResponse struct {
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

// ErrorResponse 通用错误响应
type ErrorResponse struct {
	Msg string `json:"msg"`
}

// Success 响应成功
func Success(w http.ResponseWriter, data any) {
	render := chix.NewRender(w)
	defer render.Release()
	render.JSON(&SuccessResponse{
		Msg:  "success",
		Data: data,
	})
}

// Error 响应错误
func Error(w http.ResponseWriter, code int, format string, args ...any) {
	render := chix.NewRender(w)
	defer render.Release()
	render.Header(chix.HeaderContentType, chix.MIMEApplicationJSONCharsetUTF8) // must before Status()
	render.Status(code)
	if len(args) > 0 {
		format = fmt.Sprintf(format, args...)
	}
	render.JSON(&ErrorResponse{
		Msg: format,
	})
}

// ErrorSystem 响应系统错误
func ErrorSystem(w http.ResponseWriter) {
	render := chix.NewRender(w)
	defer render.Release()
	render.Header(chix.HeaderContentType, chix.MIMEApplicationJSONCharsetUTF8) // must before Status()
	render.Status(http.StatusInternalServerError)
	render.JSON(&ErrorResponse{
		Msg: http.StatusText(http.StatusInternalServerError),
	})
}

// Bind 验证并绑定请求参数
func Bind[T any](r *http.Request) (*T, error) {
	req := new(T)

	// 绑定参数
	binder := chix.NewBind(r)
	defer binder.Release()
	if slices.Contains([]string{"POST", "PUT", "PATCH", "DELETE"}, strings.ToUpper(r.Method)) {
		if r.ContentLength > 0 {
			if err := binder.Body(req); err != nil {
				return nil, err
			}
		}
	}
	if err := binder.Query(req); err != nil {
		return nil, err
	}
	if err := binder.URI(req); err != nil {
		return nil, err
	}

	// 准备验证
	if reqWithPrepare, ok := any(req).(request.WithPrepare); ok {
		if err := reqWithPrepare.Prepare(r); err != nil {
			return nil, err
		}
	}
	if reqWithAuthorize, ok := any(req).(request.WithAuthorize); ok {
		if err := reqWithAuthorize.Authorize(r); err != nil {
			return nil, err
		}
	}

	vd := validator.Default().Struct(req)
	if reqWithRules, ok := any(req).(request.WithRules); ok {
		if rules := reqWithRules.Rules(r); rules != nil {
			for key, value := range rules {
				if err := vd.AddRules(key, value); err != nil {
					return nil, err
				}
			}
		}
	}
	if reqWithFilters, ok := any(req).(request.WithFilters); ok {
		if filters := reqWithFilters.Filters(r); filters != nil {
			for key, value := range filters {
				if err := vd.AddFilters(key, value); err != nil {
					return nil, err
				}
			}
		}
	}

	// 开始验证
	vd.Validate(r.Context())
	if vd.Fails() {
		return nil, errors.New(vd.Errors().One())
	}

	return req, nil
}

// Paginate 取分页条目
func Paginate[T any](r *http.Request, items []T) (pagedItems []T, total uint) {
	req, err := Bind[request.Paginate](r)
	if err != nil {
		req = &request.Paginate{
			Page:  1,
			Limit: 10,
		}
	}
	total = uint(len(items))
	start := (req.Page - 1) * req.Limit
	end := req.Page * req.Limit

	if total == 0 {
		return []T{}, 0
	}
	if start > total {
		return []T{}, total
	}
	if end > total {
		end = total
	}

	return items[start:end], total
}
