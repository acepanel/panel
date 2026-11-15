package route

import (
	"net/http"

	"github.com/acepanel/panel/v3/internal/service"
)

// ToolboxBenchmarkRoutes 工具箱-跑分 路由
func ToolboxBenchmarkRoutes(toolboxBenchmarkService *service.ToolboxBenchmarkService) Endpoints {
	svc := toolboxBenchmarkService

	return Endpoints{
		{Method: http.MethodGet, Path: "/api/toolbox_benchmark/machine", Handler: svc.Machine},
		{Method: http.MethodPost, Path: "/api/toolbox_benchmark/test", Handler: svc.Test},
	}
}
