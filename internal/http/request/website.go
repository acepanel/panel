package request

import "github.com/tnborg/panel/pkg/types"

type WebsiteDefaultConfig struct {
	Index string `json:"index" form:"index" validate:"required"`
	Stop  string `json:"stop" form:"stop" validate:"required"`
}

type WebsiteCreate struct {
	Name       string   `form:"name" json:"name" validate:"required|notExists:websites,name"`
	Listens    []string `form:"listens" json:"listens" validate:"required|isSlice"`
	Domains    []string `form:"domains" json:"domains" validate:"required|isSlice"`
	Path       string   `form:"path" json:"path"`
	PHP        int      `form:"php" json:"php"`
	DB         bool     `form:"db" json:"db"`
	DBType     string   `form:"db_type" json:"db_type" validate:"requiredIf:DB,true"`
	DBName     string   `form:"db_name" json:"db_name" validate:"requiredIf:DB,true"`
	DBUser     string   `form:"db_user" json:"db_user" validate:"requiredIf:DB,true"`
	DBPassword string   `form:"db_password" json:"db_password" validate:"requiredIf:DB,true"`
	Remark     string   `form:"remark" json:"remark"`
	Type       string   // 迁移用的
}

type WebsiteDelete struct {
	ID   uint `form:"id" json:"id" validate:"required|exists:websites,id"`
	Path bool `form:"path" json:"path"`
	DB   bool `form:"db" json:"db"`
}

type WebsiteUpdate struct {
	ID                uint                  `form:"id" json:"id" validate:"required|exists:websites,id"`
	Listens           []types.WebsiteListen `form:"listens" json:"listens" validate:"required|isSlice"`
	Domains           []string              `form:"domains" json:"domains" validate:"required|isSlice"`
	HTTPS             bool                  `form:"https" json:"https"`
	OCSP              bool                  `form:"ocsp" json:"ocsp"`
	HSTS              bool                  `form:"hsts" json:"hsts"`
	HTTPRedirect      bool                  `form:"http_redirect" json:"http_redirect"`
	OpenBasedir       bool                  `form:"open_basedir" json:"open_basedir"`
	Index             []string              `form:"index" json:"index" validate:"required|isSlice"`
	Path              string                `form:"path" json:"path" validate:"required"` // 网站目录
	Root              string                `form:"root" json:"root" validate:"required"` // 运行目录
	Raw               string                `form:"raw" json:"raw"`
	Rewrite           string                `form:"rewrite" json:"rewrite"`
	PHP               int                   `form:"php" json:"php"`
	SSLCertificate    string                `form:"ssl_certificate" json:"ssl_certificate" validate:"requiredIf:HTTPS,true"`
	SSLCertificateKey string                `form:"ssl_certificate_key" json:"ssl_certificate_key" validate:"requiredIf:HTTPS,true"`
}

type WebsiteUpdateRemark struct {
	ID     uint   `form:"id" json:"id" validate:"required|exists:websites,id"`
	Remark string `form:"remark" json:"remark"`
}

type WebsiteUpdateStatus struct {
	ID     uint `json:"id" form:"id" validate:"required|exists:websites,id"`
	Status bool `json:"status" form:"status"`
}

type WebsiteUpdateCert struct {
	Name string `json:"name" validate:"required|exists:websites,name"`
	Cert string `json:"cert" validate:"required"`
	Key  string `json:"key" validate:"required"`
}
