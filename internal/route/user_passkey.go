package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// UserPasskeyRoutes 通行密钥管理路由
func UserPasskeyRoutes(userPasskeyService *service.UserPasskeyService) Endpoints {
	svc := userPasskeyService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/user_passkeys", Handler: svc.List, Summary: "获取通行密钥列表", Tags: []string{"通行密钥"}, Document: Describe[request.UserPasskeyList, service.Envelope[service.Page[*biz.UserPasskey]]]()},
		{Method: http.MethodGet, Path: "/api/user_passkeys/supported", Handler: svc.Supported, Summary: "是否支持通行密钥", Tags: []string{"通行密钥"}},
		{Method: http.MethodDelete, Path: "/api/user_passkeys/{id}", Handler: svc.Delete, Summary: "删除通行密钥", Tags: []string{"通行密钥"}, Document: DescribeReq[request.UserPasskeyDelete]()},
	}
}
