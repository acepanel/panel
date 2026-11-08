// Package common 承载各应用共用的 HTTP 处理逻辑
package common

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/systemctl"
)

// ServeConfig 返回配置文件原始内容
func ServeConfig(w http.ResponseWriter, path string) {
	config, _ := io.Read(path)

	service.Success(w, config)
}

// SaveConfig 绑定请求后写入配置文件并重启对应服务，app 是操作日志里的应用标识
func SaveConfig(w http.ResponseWriter, r *http.Request, log *slog.Logger, t *gotext.Locale, app, path string, perm os.FileMode, unit string) {
	req, err := service.Bind[request.AppUpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	WriteConfig(w, r, log, t, app, path, req.Config, perm, unit)
}

// WriteConfig 写入配置文件并重启对应服务，app 是操作日志里的应用标识
func WriteConfig(w http.ResponseWriter, r *http.Request, log *slog.Logger, t *gotext.Locale, app, path, content string, perm os.FileMode, unit string) {
	if err := io.Write(path, content, perm); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err := systemctl.Restart(r.Context(), unit); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	log.Info(t.Get("app config saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", app), slog.String("path", path))

	service.Success(w, nil)
}
