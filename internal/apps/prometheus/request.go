package prometheus

type UpdateConfig struct {
	Config string `form:"config" json:"config" validate:"required"`
}

// ConfigTune Prometheus 全局配置调整
type ConfigTune struct {
	ScrapeInterval     string `form:"scrape_interval" json:"scrape_interval"`
	EvaluationInterval string `form:"evaluation_interval" json:"evaluation_interval"`
	ScrapeTimeout      string `form:"scrape_timeout" json:"scrape_timeout"`
}

// Exporter Prometheus Exporter 信息
type Exporter struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Installed   bool   `json:"installed"`
	Running     bool   `json:"running"`
}

// ExporterSlug Exporter 操作请求
type ExporterSlug struct {
	Slug string `form:"slug" json:"slug" validate:"required"`
}

// ExporterConfig Exporter 配置更新请求
type ExporterConfig struct {
	Config string `form:"config" json:"config" validate:"required"`
}
