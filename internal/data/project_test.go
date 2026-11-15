package data

import (
	"io"
	"strings"
	"testing"

	"github.com/coreos/go-systemd/v22/unit"
	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"

	"github.com/acepanel/panel/v3/internal/request"
	"github.com/acepanel/panel/v3/pkg/types"
)

func serializeUnitForTest(t *testing.T, options []*unit.UnitOption) string {
	t.Helper()
	content, err := io.ReadAll(unit.Serialize(options))
	must.NoError(t, err)
	return string(content)
}

func TestProjectCreateUnitOptions(t *testing.T) {
	r := &projectRepo{}

	got := serializeUnitForTest(t, r.createUnitOptions(&request.ProjectCreate{
		Name: "c1", Type: types.ProjectTypeGeneral, Description: "minimal", RootDir: "opt/app",
	}))
	check.DeepEqual(t, got, `[Unit]
Description=minimal
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/app
Restart=on-failure

[Install]
WantedBy=multi-user.target
`)

	got = serializeUnitForTest(t, r.createUnitOptions(&request.ProjectCreate{
		Name: "c2", Type: types.ProjectTypeJava, Description: "java app", RootDir: "/opt/java", WorkingDir: "opt/java/bin",
		ExecStart: "/usr/bin/java -jar app.jar", User: "www", Restart: "always",
		Environments: []types.KV{{Key: "JAVA_OPTS", Value: `-Xmx512m -Dname="a b"`}, {Key: "PATH", Value: `C:\bin`}},
	}))
	check.DeepEqual(t, got, `[Unit]
Description=java app
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/java/bin
ExecStart=/usr/bin/java -jar app.jar
User=www
Restart=always
SuccessExitStatus=143
Environment="JAVA_OPTS=-Xmx512m -Dname=\"a b\""
Environment="PATH=C:\\bin"

[Install]
WantedBy=multi-user.target
`)
}

func TestProjectUpdateUnitOptions(t *testing.T) {
	r := &projectRepo{}

	got := serializeUnitForTest(t, r.unitOptions(&request.ProjectUpdate{Name: "u1", Description: "minimal update", RootDir: "opt/u1"}, nil))
	check.DeepEqual(t, got, `[Unit]
Description=minimal update
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/u1
Restart=on-failure

[Install]
WantedBy=multi-user.target
`)

	// 未托管的配置项与自定义段落保留，旧版本写入的 ProtectTmp 被清掉
	existing, err := unit.DeserializeOptions(strings.NewReader(`[Unit]
Description=old
After=network.target
StartLimitIntervalSec=0

[Service]
Type=simple
SuccessExitStatus=143
ProtectTmp=true
LimitNOFILE=65535
ExecStart=/bin/old

[Socket]
ListenStream=8080

[Install]
WantedBy=multi-user.target
Alias=foo.service
`))
	must.NoError(t, err)

	got = serializeUnitForTest(t, r.unitOptions(&request.ProjectUpdate{
		Name: "u2", Description: "full update", RootDir: "/opt/u2", WorkingDir: "/opt/u2/run",
		ExecStartPre: "/bin/pre", ExecStartPost: "/bin/post", ExecStart: "/bin/start", ExecStop: "/bin/stop", ExecReload: "/bin/reload",
		User: "www", Restart: "always", RestartSec: "5s", RestartMax: 3, TimeoutStartSec: 30, TimeoutStopSec: 60,
		Environments:   []types.KV{{Key: "A", Value: "1"}, {Key: "B", Value: "x y"}},
		StandardOutput: "journal", StandardError: "append:/var/log/u2.err",
		Requires: []string{"mysqld.service"}, Wants: []string{"redis.service", "nginx.service"},
		After: []string{"network.target", "mysqld.service"}, Before: []string{"multi-user.target"},
		MemoryLimit: 512 * 1024 * 1024, CPUQuota: "50%",
		NoNewPrivileges: true, ProtectTmp: true, ProtectHome: true, ProtectSystem: "strict",
		ReadWritePaths: []string{"/opt/u2", "/var/log"}, ReadOnlyPaths: []string{"/etc"},
	}, existing))
	check.DeepEqual(t, got, `[Unit]
Description=full update
Requires=mysqld.service
Wants=redis.service
Wants=nginx.service
After=network.target
After=mysqld.service
Before=multi-user.target
StartLimitIntervalSec=0

[Service]
Type=simple
WorkingDirectory=/opt/u2/run
ExecStartPre=/bin/pre
ExecStart=/bin/start
ExecStartPost=/bin/post
ExecStop=/bin/stop
ExecReload=/bin/reload
User=www
Restart=always
RestartSec=5s
StartLimitBurst=3
TimeoutStartSec=30
TimeoutStopSec=60
Environment="A=1"
Environment="B=x y"
StandardOutput=journal
StandardError=append:/var/log/u2.err
MemoryLimit=512M
CPUQuota=50%
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=strict
ReadWritePaths=/opt/u2
ReadWritePaths=/var/log
ReadOnlyPaths=/etc
SuccessExitStatus=143
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
Alias=foo.service

[Socket]
ListenStream=8080
`)
}

func TestProjectParseProtectTmp(t *testing.T) {
	r := &projectRepo{}

	for _, name := range []string{"PrivateTmp", "ProtectTmp"} {
		detail := new(types.ProjectDetail)
		r.parseServiceSection(detail, unit.NewUnitOption("Service", name, "true"))
		check.True(t, detail.ProtectTmp)
	}
}
