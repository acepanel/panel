package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/types"
)

// CertRoutes 证书、DNS、账户路由
func CertRoutes(certAccountService *service.CertAccountService, certDNSService *service.CertDNSService, certService *service.CertService) Endpoints {
	cert := certService
	certDNS := certDNSService
	certAccount := certAccountService

	return Endpoints{
		// 顶层选项
		{Method: http.MethodGet, Path: "/api/cert/ca_providers", Handler: cert.CAProviders, Summary: "CA 提供商列表", Tags: []string{"证书"}},
		{Method: http.MethodGet, Path: "/api/cert/dns_providers", Handler: cert.DNSProviders, Summary: "DNS 提供商列表", Tags: []string{"证书"}},
		{Method: http.MethodGet, Path: "/api/cert/algorithms", Handler: cert.Algorithms, Summary: "密钥算法列表", Tags: []string{"证书"}},
		// 证书
		{Method: http.MethodGet, Path: "/api/cert/cert", Handler: cert.List, Summary: "证书列表", Tags: []string{"证书"},
			Document: Describe[request.Paginate, service.Envelope[service.Page[*types.CertList]]]()},
		{Method: http.MethodPost, Path: "/api/cert/cert", Handler: cert.Create, Summary: "创建证书", Tags: []string{"证书"},
			Document: Describe[request.CertCreate, service.Envelope[biz.Cert]]()},
		{Method: http.MethodPost, Path: "/api/cert/cert/upload", Handler: cert.Upload, Summary: "上传证书", Tags: []string{"证书"},
			Document: Describe[request.CertUpload, service.Envelope[biz.Cert]]()},
		{Method: http.MethodPut, Path: "/api/cert/cert/{id}", Handler: cert.Update, Summary: "更新证书", Tags: []string{"证书"},
			Document: DescribeReq[request.CertUpdate]()},
		{Method: http.MethodGet, Path: "/api/cert/cert/{id}", Handler: cert.Get, Summary: "获取证书", Tags: []string{"证书"},
			Document: Describe[request.ID, service.Envelope[biz.Cert]]()},
		{Method: http.MethodDelete, Path: "/api/cert/cert/{id}", Handler: cert.Delete, Summary: "删除证书", Tags: []string{"证书"},
			Document: DescribeReq[request.ID]()},
		{Method: http.MethodPost, Path: "/api/cert/cert/{id}/obtain_auto", Handler: cert.ObtainAuto, Summary: "自动签发证书", Tags: []string{"证书"},
			Document: DescribeReq[request.ID]()},
		{Method: http.MethodPost, Path: "/api/cert/cert/{id}/obtain_self_signed", Handler: cert.ObtainSelfSigned, Summary: "签发自签名证书", Tags: []string{"证书"},
			Document: DescribeReq[request.ID]()},
		{Method: http.MethodPost, Path: "/api/cert/cert/{id}/renew", Handler: cert.Renew, Summary: "续签证书", Tags: []string{"证书"},
			Document: DescribeReq[request.ID]()},
		{Method: http.MethodPost, Path: "/api/cert/cert/{id}/deploy", Handler: cert.Deploy, Summary: "部署证书", Tags: []string{"证书"},
			Document: DescribeReq[request.CertDeploy]()},
		// DNS
		{Method: http.MethodGet, Path: "/api/cert/dns", Handler: certDNS.List, Summary: "DNS 列表", Tags: []string{"证书"},
			Document: Describe[request.Paginate, service.Envelope[service.Page[*biz.CertDNS]]]()},
		{Method: http.MethodPost, Path: "/api/cert/dns", Handler: certDNS.Create, Summary: "创建 DNS", Tags: []string{"证书"},
			Document: Describe[request.CertDNSCreate, service.Envelope[biz.CertDNS]]()},
		{Method: http.MethodPut, Path: "/api/cert/dns/{id}", Handler: certDNS.Update, Summary: "更新 DNS", Tags: []string{"证书"},
			Document: DescribeReq[request.CertDNSUpdate]()},
		{Method: http.MethodGet, Path: "/api/cert/dns/{id}", Handler: certDNS.Get, Summary: "获取 DNS", Tags: []string{"证书"},
			Document: Describe[request.ID, service.Envelope[biz.CertDNS]]()},
		{Method: http.MethodDelete, Path: "/api/cert/dns/{id}", Handler: certDNS.Delete, Summary: "删除 DNS", Tags: []string{"证书"},
			Document: DescribeReq[request.ID]()},
		// 账户
		{Method: http.MethodGet, Path: "/api/cert/account", Handler: certAccount.List, Summary: "账户列表", Tags: []string{"证书"},
			Document: Describe[request.Paginate, service.Envelope[service.Page[*biz.CertAccount]]]()},
		{Method: http.MethodPost, Path: "/api/cert/account", Handler: certAccount.Create, Summary: "创建账户", Tags: []string{"证书"},
			Document: Describe[request.CertAccountCreate, service.Envelope[biz.CertAccount]]()},
		{Method: http.MethodPut, Path: "/api/cert/account/{id}", Handler: certAccount.Update, Summary: "更新账户", Tags: []string{"证书"},
			Document: DescribeReq[request.CertAccountUpdate]()},
		{Method: http.MethodGet, Path: "/api/cert/account/{id}", Handler: certAccount.Get, Summary: "获取账户", Tags: []string{"证书"},
			Document: Describe[request.ID, service.Envelope[biz.CertAccount]]()},
		{Method: http.MethodDelete, Path: "/api/cert/account/{id}", Handler: certAccount.Delete, Summary: "删除账户", Tags: []string{"证书"},
			Document: DescribeReq[request.ID]()},
	}
}
