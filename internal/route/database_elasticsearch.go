package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/db"
)

// DatabaseElasticsearchRoutes Elasticsearch 路由
func DatabaseElasticsearchRoutes(databaseElasticsearchService *service.DatabaseElasticsearchService) Endpoints {
	svc := databaseElasticsearchService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/database_elasticsearch/indices", Handler: svc.Indices,
			Summary: "获取索引列表", Tags: []string{"Elasticsearch"},
			Document: Describe[request.DatabaseESIndices, service.Envelope[[]db.ESIndex]]()},
		{Method: http.MethodPost, Path: "/api/database_elasticsearch/index", Handler: svc.IndexCreate,
			Summary: "创建索引", Tags: []string{"Elasticsearch"},
			Document: DescribeReq[request.DatabaseESIndexCreate]()},
		{Method: http.MethodDelete, Path: "/api/database_elasticsearch/index", Handler: svc.IndexDelete,
			Summary: "删除索引", Tags: []string{"Elasticsearch"},
			Document: DescribeReq[request.DatabaseESIndexDelete]()},
		{Method: http.MethodGet, Path: "/api/database_elasticsearch/data", Handler: svc.Data,
			Summary: "获取文档列表", Tags: []string{"Elasticsearch"},
			Document: Describe[request.DatabaseESData, service.Envelope[service.Page[db.ESDocument]]]()},
		{Method: http.MethodGet, Path: "/api/database_elasticsearch/document", Handler: svc.DocumentGet,
			Summary: "获取文档", Tags: []string{"Elasticsearch"},
			Document: Describe[request.DatabaseESDocumentGet, service.Envelope[db.ESDocument]]()},
		{Method: http.MethodPost, Path: "/api/database_elasticsearch/document", Handler: svc.DocumentSet,
			Summary: "设置文档", Tags: []string{"Elasticsearch"},
			Document: DescribeReq[request.DatabaseESDocumentSet]()},
		{Method: http.MethodDelete, Path: "/api/database_elasticsearch/document", Handler: svc.DocumentDelete,
			Summary: "删除文档", Tags: []string{"Elasticsearch"},
			Document: DescribeReq[request.DatabaseESDocumentDelete]()},
	}
}
