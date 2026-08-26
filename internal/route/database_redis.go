package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/db"
)

// DatabaseRedisRoutes Redis 路由
func DatabaseRedisRoutes(databaseRedisService *service.DatabaseRedisService) Endpoints {
	svc := databaseRedisService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/database_redis/databases", Handler: svc.Databases,
			Summary: "获取数据库数量", Tags: []string{"Redis"},
			Document: DescribeReq[request.DatabaseRedisDatabases]()},
		{Method: http.MethodGet, Path: "/api/database_redis/data", Handler: svc.Data,
			Summary: "获取键值列表", Tags: []string{"Redis"},
			Document: Describe[request.DatabaseRedisData, service.Envelope[service.Page[db.RedisKV]]]()},
		{Method: http.MethodGet, Path: "/api/database_redis/key", Handler: svc.KeyGet,
			Summary: "获取键值", Tags: []string{"Redis"},
			Document: Describe[request.DatabaseRedisKeyGet, service.Envelope[db.RedisKV]]()},
		{Method: http.MethodPost, Path: "/api/database_redis/key", Handler: svc.KeySet,
			Summary: "设置键值", Tags: []string{"Redis"},
			Document: DescribeReq[request.DatabaseRedisKeySet]()},
		{Method: http.MethodDelete, Path: "/api/database_redis/key", Handler: svc.KeyDelete,
			Summary: "删除键值", Tags: []string{"Redis"},
			Document: DescribeReq[request.DatabaseRedisKeyDelete]()},
		{Method: http.MethodPost, Path: "/api/database_redis/key/ttl", Handler: svc.KeyTTL,
			Summary: "设置键值过期时间", Tags: []string{"Redis"},
			Document: DescribeReq[request.DatabaseRedisKeyTTL]()},
		{Method: http.MethodPost, Path: "/api/database_redis/key/rename", Handler: svc.KeyRename,
			Summary: "重命名键值", Tags: []string{"Redis"},
			Document: DescribeReq[request.DatabaseRedisKeyRename]()},
		{Method: http.MethodPost, Path: "/api/database_redis/clear", Handler: svc.Clear,
			Summary: "清空数据库", Tags: []string{"Redis"},
			Document: DescribeReq[request.DatabaseRedisClear]()},
	}
}
