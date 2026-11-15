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
	"github.com/acepanel/panel/v3/pkg/clientip"
)

// defaultValidator Bind 使用的校验器
var defaultValidator = validator.Default()

// errorResponse 通用错误响应
type errorResponse struct {
	Msg string `json:"msg"`
}

// SetValidator 设置 Bind 使用的校验器
func SetValidator(v *validator.Validator) {
	defaultValidator = v
}

// Success 响应成功
func Success(w http.ResponseWriter, data any) {
	type successResponse struct {
		Msg  string `json:"msg"`
		Data any    `json:"data"`
	}

	render := chix.NewRender(w)
	defer render.Release()
	render.JSON(&successResponse{
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
	render.JSON(&errorResponse{
		Msg: format,
	})
}

// ErrorSystem 响应系统错误
func ErrorSystem(w http.ResponseWriter) {
	render := chix.NewRender(w)
	defer render.Release()
	render.Header(chix.HeaderContentType, chix.MIMEApplicationJSONCharsetUTF8) // must before Status()
	render.Status(http.StatusInternalServerError)
	render.JSON(&errorResponse{
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

	vd, err := defaultValidator.Struct(req)
	if err != nil {
		return nil, err
	}

	// 开始验证
	if err = vd.Validate(r.Context()); err != nil {
		if errs, ok := validator.AsErrors(err); ok {
			return nil, errors.New(errs.One())
		}
		return nil, err
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

// clientIP 提取客户端 IP
func clientIP(r *http.Request, ipHeader string) string {
	return clientip.FromRequest(r, ipHeader)
}
