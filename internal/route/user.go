package route

import (
	"net/http"
	"time"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
)

// UserRoutes 用户与通行密钥认证、用户管理路由
func UserRoutes(userPasskeyService *service.UserPasskeyService, userService *service.UserService) Endpoints {
	svc := userService
	passkey := userPasskeyService

	return Endpoints{
		// 认证
		{Method: http.MethodGet, Path: "/api/user/key", Handler: svc.GetKey, Summary: "获取登录公钥", Tags: []string{"用户"}, Public: true},
		{Method: http.MethodGet, Path: "/api/user/captcha", Handler: svc.GetCaptcha, Summary: "获取登录验证码", Tags: []string{"用户"}, Public: true},
		{Method: http.MethodPost, Path: "/api/user/login", Handler: svc.Login, Summary: "登录", Tags: []string{"用户"}, Document: DescribeReq[request.UserLogin](), Public: true, Throttle: &ThrottleRule{Tokens: 5, Interval: time.Minute}},
		{Method: http.MethodPost, Path: "/api/user/logout", Handler: svc.Logout, Summary: "登出", Tags: []string{"用户"}, Public: true},
		{Method: http.MethodGet, Path: "/api/user/is_login", Handler: svc.IsLogin, Summary: "是否已登录", Tags: []string{"用户"}, Public: true},
		{Method: http.MethodGet, Path: "/api/user/is_2fa", Handler: svc.IsTwoFA, Summary: "是否开启两步验证", Tags: []string{"用户"}, Document: DescribeReq[request.UserIsTwoFA](), Public: true},
		{Method: http.MethodGet, Path: "/api/user/info", Handler: svc.Info, Summary: "获取当前用户信息", Tags: []string{"用户"}},
		// 通行密钥
		{Method: http.MethodGet, Path: "/api/user/passkey/enabled", Handler: passkey.Enabled, Summary: "是否启用通行密钥", Tags: []string{"通行密钥"}, Public: true},
		{Method: http.MethodPost, Path: "/api/user/passkey/register", Handler: passkey.BeginRegister, Summary: "开始注册通行密钥", Tags: []string{"通行密钥"}},
		{Method: http.MethodPut, Path: "/api/user/passkey/register", Handler: passkey.FinishRegister, Summary: "完成注册通行密钥", Tags: []string{"通行密钥"}, Document: DescribeResp[service.Envelope[biz.UserPasskey]]()},
		{Method: http.MethodPost, Path: "/api/user/passkey/login", Handler: passkey.BeginLogin, Summary: "开始通行密钥登录", Tags: []string{"通行密钥"}, Public: true, Throttle: &ThrottleRule{Tokens: 5, Interval: time.Minute}},
		{Method: http.MethodPut, Path: "/api/user/passkey/login", Handler: passkey.FinishLogin, Summary: "完成通行密钥登录", Tags: []string{"通行密钥"}, Public: true},
		// 用户管理
		{Method: http.MethodGet, Path: "/api/users", Handler: svc.List, Summary: "获取用户列表", Tags: []string{"用户"}, Document: Describe[request.Paginate, service.Envelope[service.Page[*biz.User]]]()},
		{Method: http.MethodPost, Path: "/api/users", Handler: svc.Create, Summary: "创建用户", Tags: []string{"用户"}, Document: Describe[request.UserCreate, service.Envelope[biz.User]]()},
		{Method: http.MethodPost, Path: "/api/users/{id}/username", Handler: svc.UpdateUsername, Summary: "修改用户名", Tags: []string{"用户"}, Document: DescribeReq[request.UserUpdateUsername]()},
		{Method: http.MethodPost, Path: "/api/users/{id}/password", Handler: svc.UpdatePassword, Summary: "修改密码", Tags: []string{"用户"}, Document: DescribeReq[request.UserUpdatePassword]()},
		{Method: http.MethodPost, Path: "/api/users/{id}/email", Handler: svc.UpdateEmail, Summary: "修改邮箱", Tags: []string{"用户"}, Document: DescribeReq[request.UserUpdateEmail]()},
		{Method: http.MethodGet, Path: "/api/users/{id}/2fa", Handler: svc.GenerateTwoFA, Summary: "生成两步验证密钥", Tags: []string{"用户"}, Document: DescribeReq[request.UserID]()},
		{Method: http.MethodPost, Path: "/api/users/{id}/2fa", Handler: svc.UpdateTwoFA, Summary: "更新两步验证", Tags: []string{"用户"}, Document: DescribeReq[request.UserUpdateTwoFA]()},
		{Method: http.MethodDelete, Path: "/api/users/{id}", Handler: svc.Delete, Summary: "删除用户", Tags: []string{"用户"}, Document: DescribeReq[request.UserID]()},
	}
}
