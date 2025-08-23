package service

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gookit/validate"

	"github.com/tnborg/panel/internal/http/request"
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
func Success[T any](c fiber.Ctx, data T) error {
	return c.JSON(&SuccessResponse{
		Msg:  "success",
		Data: data,
	})
}

// Error 响应错误
func Error(c fiber.Ctx, code int, format string, args ...any) error {
	if len(args) > 0 {
		format = fmt.Sprintf(format, args...)
	}
	return c.Status(code).JSON(&ErrorResponse{
		Msg: format,
	})
}

// ErrorSystem 响应系统错误
func ErrorSystem(c fiber.Ctx) error {
	return c.Status(http.StatusInternalServerError).JSON(&ErrorResponse{
		Msg: http.StatusText(http.StatusInternalServerError),
	})
}

// Bind 验证并绑定请求参数
func Bind[T any](c fiber.Ctx) (*T, error) {
	req := new(T)

	// 绑定参数
	if slices.Contains([]string{"POST", "PUT", "PATCH", "DELETE"}, strings.ToUpper(c.Method())) {
		if len(c.Body()) > 0 {
			if err := c.BodyParser(req); err != nil {
				return nil, err
			}
		}
	}
	if err := c.QueryParser(req); err != nil {
		return nil, err
	}
	if err := c.ParamsParser(req); err != nil {
		return nil, err
	}

	// 准备验证
	df, err := validate.FromStruct(req)
	if err != nil {
		return nil, err
	}
	v := df.Create()

	// 创建兼容的 *http.Request 用于现有接口
	httpReq := &http.Request{
		Method: c.Method(),
		URL: &url.URL{
			Path:     c.Path(),
			RawQuery: string(c.Context().QueryArgs().QueryString()),
		},
		Header: make(http.Header),
	}

	// 复制请求头
	c.Context().Request.Header.VisitAll(func(key, value []byte) {
		httpReq.Header.Set(string(key), string(value))
	})

	if reqWithPrepare, ok := any(req).(request.WithPrepare); ok {
		if err = reqWithPrepare.Prepare(httpReq); err != nil {
			return nil, err
		}
	}
	if reqWithAuthorize, ok := any(req).(request.WithAuthorize); ok {
		if err = reqWithAuthorize.Authorize(httpReq); err != nil {
			return nil, err
		}
	}
	if reqWithRules, ok := any(req).(request.WithRules); ok {
		if rules := reqWithRules.Rules(httpReq); rules != nil {
			for key, value := range rules {
				v.StringRule(key, value)
			}
		}
	}
	if reqWithFilters, ok := any(req).(request.WithFilters); ok {
		if filters := reqWithFilters.Filters(httpReq); filters != nil {
			v.FilterRules(filters)
		}
	}
	if reqWithMessages, ok := any(req).(request.WithMessages); ok {
		if messages := reqWithMessages.Messages(httpReq); messages != nil {
			v.AddMessages(messages)
		}
	}

	// 开始验证
	if v.Validate() && v.IsSuccess() {
		return req, nil
	}

	return nil, v.Errors.OneError()
}

// Paginate 取分页条目
func Paginate[T any](c fiber.Ctx, items []T) (pagedItems []T, total uint) {
	req, err := Bind[request.Paginate](c)
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
