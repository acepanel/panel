package geoip

import (
	"log/slog"

	"github.com/acepanel/panel/v3/pkg/ipdb"
)

// GeoIP IP 地理位置解析
type GeoIP struct {
	db             *ipdb.Reader
	idxCountry     int
	idxRegion      int
	idxCity        int
	idxCountryCode int
	idxISP         int
}

// GeoResult 地理位置查询结果
type GeoResult struct {
	Country     string
	Region      string
	City        string
	CountryCode string
	ISP         string
}

// NewGeoIP 加载 .ipdb 文件，返回 GeoIP 实例
func NewGeoIP(path string) (*GeoIP, error) {
	db, err := ipdb.Open(path)
	if err != nil {
		return nil, err
	}
	g := &GeoIP{db: db}
	g.cacheFieldIndices()
	return g, nil
}

// Reload 重新加载 IP 数据库文件
func (g *GeoIP) Reload(path string) error {
	if err := g.db.Reload(path); err != nil {
		return err
	}
	g.cacheFieldIndices()
	return nil
}

// Close 释放数据库资源
func (g *GeoIP) Close() error {
	if g == nil {
		return nil
	}
	return g.db.Close()
}

// Lookup 查询 IP 的地理位置，失败返回空结果
func (g *GeoIP) Lookup(ip string) GeoResult {
	if g == nil || g.db == nil {
		return GeoResult{}
	}

	fields, err := g.db.Find(ip, "CN")
	if err != nil {
		slog.Debug("geoip lookup failed", slog.String("ip", ip), slog.Any("err", err))
		return GeoResult{}
	}

	var r GeoResult
	if g.idxCountry >= 0 {
		r.Country = fields[g.idxCountry]
	}
	if g.idxRegion >= 0 {
		r.Region = fields[g.idxRegion]
	}
	if g.idxCity >= 0 {
		r.City = fields[g.idxCity]
	}
	if g.idxCountryCode >= 0 {
		r.CountryCode = fields[g.idxCountryCode]
	}
	if g.idxISP >= 0 {
		r.ISP = fields[g.idxISP]
	}
	return r
}

// cacheFieldIndices 缓存字段索引，避免每次查询遍历
func (g *GeoIP) cacheFieldIndices() {
	g.idxCountry = -1
	g.idxRegion = -1
	g.idxCity = -1
	g.idxCountryCode = -1
	g.idxISP = -1
	for i, f := range g.db.Fields() {
		switch f {
		case "country_name":
			g.idxCountry = i
		case "region_name":
			g.idxRegion = i
		case "city_name":
			g.idxCity = i
		case "country_code":
			g.idxCountryCode = i
		case "isp_domain":
			g.idxISP = i
		}
	}
}
