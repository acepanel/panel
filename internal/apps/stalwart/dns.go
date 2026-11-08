package stalwart

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/libdns/libdns"
	lop "github.com/samber/lo/parallel"
	"golang.org/x/net/publicsuffix"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/internal/service"
	"github.com/acepanel/panel/v3/pkg/acme"
)

var quotedRe = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)

// parseZoneFile 解析 Stalwart 生成的 BIND 区域文本，括号包裹的多行 TXT 会先并成一行
func parseZoneFile(zone string, domain string) []DNSRecord {
	var logical []string
	var buf strings.Builder
	depth := 0
	for line := range strings.SplitSeq(zone, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		buf.WriteString(" ")
		buf.WriteString(line)
		depth += strings.Count(line, "(") - strings.Count(line, ")")
		if depth <= 0 {
			logical = append(logical, strings.TrimSpace(buf.String()))
			buf.Reset()
			depth = 0
		}
	}

	seen := map[string]int{}
	var records []DNSRecord
	for _, entry := range logical {
		fields := strings.Fields(entry)
		if len(fields) < 4 || fields[1] != "IN" {
			continue
		}
		name := strings.TrimSuffix(fields[0], ".")
		typ := fields[2]
		rest := strings.TrimSpace(strings.TrimPrefix(entry, strings.Join(fields[:3], " ")))
		rec := DNSRecord{Type: typ, Name: name, Group: "optional"}

		switch typ {
		case "TXT":
			var parts []string
			for _, m := range quotedRe.FindAllStringSubmatch(rest, -1) {
				parts = append(parts, m[1])
			}
			rec.Value = strings.Join(parts, "")
			lower := strings.ToLower(rec.Value)
			if strings.HasPrefix(lower, "v=spf1") || strings.HasPrefix(lower, "v=dkim1") || strings.HasPrefix(lower, "v=dmarc1") {
				rec.Group = "required"
			}
		case "MX":
			if len(fields) < 5 {
				continue
			}
			rec.priority = parseUint16(fields[3])
			rec.target = fields[4]
			rec.Value = fields[3] + " " + fields[4]
			rec.Group = "required"
		case "SRV":
			if len(fields) < 7 {
				continue
			}
			rec.priority, rec.weight, rec.port = parseUint16(fields[3]), parseUint16(fields[4]), parseUint16(fields[5])
			rec.target = fields[6]
			rec.Value = strings.Join(fields[3:7], " ")
		case "CNAME":
			rec.target = fields[3]
			rec.Value = fields[3]
		default:
			continue
		}

		id := typ + ":" + name
		if n := seen[id]; n > 0 {
			id = fmt.Sprintf("%s#%d", id, n)
		}
		seen[typ+":"+name]++
		rec.ID = id
		_ = domain
		records = append(records, rec)
	}
	return records
}

func parseUint16(v string) uint16 {
	n, _ := strconv.ParseUint(v, 10, 16)
	return uint16(n)
}

// checkRecords 并发查询每条记录当前的解析结果并标注状态
func checkRecords(ctx context.Context, records []DNSRecord) []DNSRecord {
	resolver := net.DefaultResolver
	return lop.Map(records, func(rec DNSRecord, _ int) DNSRecord {
		lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		rec.Status, rec.Found = lookupRecord(lookupCtx, resolver, rec)
		return rec
	})
}

func lookupRecord(ctx context.Context, resolver *net.Resolver, rec DNSRecord) (string, string) {
	normalize := func(v string) string { return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(v), ".")) }
	switch rec.Type {
	case "TXT":
		values, err := resolver.LookupTXT(ctx, rec.Name)
		if err != nil {
			return lookupStatus(err), ""
		}
		want := strings.ReplaceAll(rec.Value, " ", "")
		prefix, _, _ := strings.Cut(strings.ToLower(rec.Value), ";")
		var similar []string
		for _, v := range values {
			if strings.ReplaceAll(v, " ", "") == want {
				return "ok", v
			}
			if strings.HasPrefix(strings.ToLower(v), prefix) {
				similar = append(similar, v)
			}
		}
		if len(similar) > 0 {
			return "mismatch", strings.Join(similar, "\n")
		}
		return "missing", ""
	case "MX":
		mxs, err := resolver.LookupMX(ctx, rec.Name)
		if err != nil {
			return lookupStatus(err), ""
		}
		var found []string
		for _, mx := range mxs {
			found = append(found, fmt.Sprintf("%d %s", mx.Pref, mx.Host))
			if normalize(mx.Host) == normalize(rec.target) {
				return "ok", fmt.Sprintf("%d %s", mx.Pref, mx.Host)
			}
		}
		if len(found) > 0 {
			return "mismatch", strings.Join(found, "\n")
		}
		return "missing", ""
	case "CNAME":
		cname, err := resolver.LookupCNAME(ctx, rec.Name)
		if err != nil {
			return lookupStatus(err), ""
		}
		if normalize(cname) == normalize(rec.target) {
			return "ok", cname
		}
		if normalize(cname) == normalize(rec.Name) || cname == "" {
			return "missing", ""
		}
		return "mismatch", cname
	case "SRV":
		_, srvs, err := resolver.LookupSRV(ctx, "", "", rec.Name)
		if err != nil {
			return lookupStatus(err), ""
		}
		var found []string
		for _, srv := range srvs {
			found = append(found, fmt.Sprintf("%d %d %d %s", srv.Priority, srv.Weight, srv.Port, srv.Target))
			if normalize(srv.Target) == normalize(rec.target) && srv.Port == rec.port {
				return "ok", found[len(found)-1]
			}
		}
		if len(found) > 0 {
			return "mismatch", strings.Join(found, "\n")
		}
		return "missing", ""
	}
	return "unknown", ""
}

// lookupStatus 把解析错误归为「不存在」或「无法判断」
func lookupStatus(err error) string {
	var dnsErr *net.DNSError
	if ok := asDNSError(err, &dnsErr); ok && dnsErr.IsNotFound {
		return "missing"
	}
	return "unknown"
}

// DomainDNS 返回域名需要发布的 DNS 记录及其当前解析状态
func (s *App) DomainDNS(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[DomainID](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	c, err := s.adminClient()
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	domain, err := getOne[swDomain](r.Context(), c, "Domain", req.ID)
	if err != nil {
		service.Error(w, http.StatusNotFound, "%v", err)
		return
	}
	records := checkRecords(r.Context(), parseZoneFile(domain.DNSZoneFile, domain.Name))
	zone, err := publicsuffix.EffectiveTLDPlusOne(domain.Name)
	if err != nil {
		zone = domain.Name
	}

	s.log.Info(s.t.Get("mail domain dns viewed"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", slug), slog.String("domain", domain.Name))
	service.Success(w, DNSResult{Zone: zone, Records: records})
}

// DomainDNSPublish 用面板已配置的 DNS 账号把选中的记录写进解析商
func (s *App) DomainDNSPublish(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[DomainDNSPublish](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}
	c, err := s.adminClient()
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	domain, err := getOne[swDomain](r.Context(), c, "Domain", req.ID)
	if err != nil {
		service.Error(w, http.StatusNotFound, "%v", err)
		return
	}
	dns, err := s.certDNSRepo.Get(req.DNSID)
	if err != nil {
		service.Error(w, http.StatusNotFound, "%v", err)
		return
	}
	provider, err := acme.NewDNSProvider(dns.Type, dns.Data)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	zone, err := publicsuffix.EffectiveTLDPlusOne(domain.Name)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	var recs []libdns.Record
	for _, rec := range parseZoneFile(domain.DNSZoneFile, domain.Name) {
		if !slices.Contains(req.Records, rec.ID) {
			continue
		}
		name := libdns.RelativeName(rec.Name+".", zone+".")
		switch rec.Type {
		case "TXT":
			recs = append(recs, libdns.TXT{Name: name, Text: rec.Value})
		case "MX":
			recs = append(recs, libdns.MX{Name: name, Preference: rec.priority, Target: rec.target})
		case "CNAME":
			recs = append(recs, libdns.CNAME{Name: name, Target: rec.target})
		case "SRV":
			// _imaps._tcp.example.com 拆成 service、transport 与所属名
			parts := strings.SplitN(name, ".", 3)
			if len(parts) < 2 {
				continue
			}
			owner := "@"
			if len(parts) == 3 {
				owner = parts[2]
			}
			recs = append(recs, libdns.SRV{
				Service:   strings.TrimPrefix(parts[0], "_"),
				Transport: strings.TrimPrefix(parts[1], "_"),
				Name:      owner,
				Priority:  rec.priority,
				Weight:    rec.weight,
				Port:      rec.port,
				Target:    rec.target,
			})
		}
	}
	if len(recs) == 0 {
		service.Error(w, http.StatusUnprocessableEntity, s.t.Get("no records selected"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	created, err := provider.AppendRecords(ctx, zone+".", recs)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	s.log.Info(s.t.Get("mail domain dns records published"), slog.String("type", biz.OperationTypeApp), slog.Uint64("operator_id", biz.OperatorID(r.Context())), slog.String("app", slug), slog.String("domain", domain.Name), slog.String("zone", zone), slog.Uint64("dns_id", uint64(req.DNSID)), slog.Any("records", req.Records), slog.Int("published", len(created)))
	service.Success(w, map[string]int{"published": len(created)})
}
