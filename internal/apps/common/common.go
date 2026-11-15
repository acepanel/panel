// Package common 承载各应用共用的 HTTP 处理逻辑
package common

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"

	"github.com/leonelquinteros/gotext"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/firewall"
	"github.com/acepanel/panel/v3/pkg/io"
	"github.com/acepanel/panel/v3/pkg/systemctl"
)

// ServeConfig 返回配置文件原始内容，文件不存在时视为空内容
func ServeConfig(w http.ResponseWriter, path string) {
	config, err := io.Read(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, config)
}

// SaveConfig 绑定请求后写入配置文件并让其生效，app 是操作日志里的应用标识
func SaveConfig(
	w http.ResponseWriter,
	r *http.Request,
	log *slog.Logger,
	t *gotext.Locale,
	app, path string,
	perm os.FileMode,
	apply func(context.Context) error,
) {
	req, err := service.Bind[request.AppUpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	WriteConfig(w, r, log, t, app, path, req.Config, perm, apply)
}

// WriteConfig 写入配置文件并让其生效，app 是操作日志里的应用标识
func WriteConfig(
	w http.ResponseWriter,
	r *http.Request,
	log *slog.Logger,
	t *gotext.Locale,
	app, path, content string,
	perm os.FileMode,
	apply func(context.Context) error,
) {
	if err := io.Write(path, content, perm); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	// 配置已落盘，生效过程不跟随请求取消，否则磁盘配置与运行中的服务不一致
	if err := apply(context.WithoutCancel(r.Context())); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	log.Info(t.Get("app config saved"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", app), slog.String("path", path))

	service.Success(w, nil)
}

// Restart 返回依次重启各服务的生效函数，供 SaveConfig、WriteConfig 使用
func Restart(units ...string) func(context.Context) error {
	return func(ctx context.Context) error {
		for _, unit := range units {
			if err := systemctl.Restart(ctx, unit); err != nil {
				return err
			}
		}

		return nil
	}
}

// OpenPort 防火墙开着时放行入站端口，返回防火墙是否在运行，protocol 留空表示 TCP 与 UDP
func OpenPort(ctx context.Context, port uint, protocol firewall.Protocol) (bool, error) {
	fw := firewall.NewFirewall()
	running, err := fw.Status(ctx)
	if err != nil || !running {
		return false, err
	}

	return true, fw.Port(ctx, firewall.FireInfo{
		Type:      firewall.TypeNormal,
		PortStart: port,
		PortEnd:   port,
		Protocol:  protocol,
		Strategy:  firewall.StrategyAccept,
		Direction: firewall.DirectionIn,
	}, firewall.OperationAdd)
}
