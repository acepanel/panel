package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/acepanel/panel/v3/pkg/io"
)

// ifaceStanzaPattern 匹配 iface <name> <inet|inet6> <method> 声明
var ifaceStanzaPattern = regexp.MustCompile(`^\s*iface\s+(\S+)\s+(inet6?)\s+(\S+)`)

type ifupdownBackend struct{}

type interfacesFile struct {
	path    string
	lines   []string
	stanzas []interfacesStanza
}

type interfacesStanza struct {
	name, family, method string
	start, end           int
	options              []string
}

func (b *ifupdownBackend) Name() string { return "ifupdown" }

func (b *ifupdownBackend) available(_ context.Context) bool {
	return hasCommand("ifquery") && io.Exists("/etc/network/interfaces")
}

func (b *ifupdownBackend) Load(_ context.Context, items []Interface) error {
	files, err := b.read()
	if err != nil {
		return err
	}
	for i := range items {
		file, stanzas, reason := b.find(files, items[i].Name)
		if reason != "" {
			items[i].Reason = reason
			continue
		}
		items[i].IPv4 = b.family(stanzas, "inet")
		items[i].IPv6 = b.family(stanzas, "inet6")
		for _, stanza := range stanzas {
			if mtu := stanza.value("mtu"); mtu != "" {
				items[i].ConfiguredMTU, _ = strconv.Atoi(mtu)
				break
			}
		}
		items[i].Editable = file != nil
	}
	return nil
}

func (b *ifupdownBackend) Apply(
	ctx context.Context,
	item Interface,
	config Config,
) (func(context.Context) error, error) {
	files, err := b.read()
	if err != nil {
		return nil, err
	}
	file, stanzas, reason := b.find(files, config.Name)
	if reason != "" || file == nil {
		return nil, fmt.Errorf("%w: %s", ErrValidation, reason)
	}

	// 原文件内容即快照，写回并重载即可撤销
	original := strings.Join(file.lines, "")
	rollback := func(ctx context.Context) error {
		if err := os.WriteFile(file.path, []byte(original), 0644); err != nil {
			return err
		}
		return b.reload(ctx, config.Name)
	}

	start, end := stanzas[0].start, stanzas[len(stanzas)-1].end
	lines := slices.Concat(
		file.lines[:start],
		b.render(config, b.unmanaged(stanzas, "inet"), b.unmanaged(stanzas, "inet6")),
		file.lines[end:],
	)
	if err = b.write(ctx, file.path, config.Name, strings.Join(lines, "")); err != nil {
		return nil, err
	}
	if err = b.reload(ctx, config.Name); err != nil {
		return nil, errors.Join(err, rollback(context.WithoutCancel(ctx)))
	}
	return rollback, nil
}

func (b *ifupdownBackend) read() ([]interfacesFile, error) {
	paths := []string{"/etc/network/interfaces"}
	included, _ := filepath.Glob("/etc/network/interfaces.d/*")
	paths = append(paths, included...)

	files := make([]interfacesFile, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		files = append(files, b.parse(path, string(data)))
	}
	return files, nil
}

// parse 按 iface 声明切分出各段落，保留原始行以便原样回写未改动的部分
func (b *ifupdownBackend) parse(path, content string) interfacesFile {
	lines := strings.SplitAfter(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	file := interfacesFile{path: path, lines: lines}
	for i := 0; i < len(lines); {
		match := ifaceStanzaPattern.FindStringSubmatch(lines[i])
		if len(match) == 0 {
			i++
			continue
		}
		// 段落延伸到下一个顶格的非注释行为止
		end := i + 1
		for end < len(lines) {
			trimmed := strings.TrimSpace(lines[end])
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") && lines[end][0] != ' ' && lines[end][0] != '\t' {
				break
			}
			end++
		}
		file.stanzas = append(file.stanzas, interfacesStanza{
			name: match[1], family: match[2], method: match[3], start: i, end: end,
			options: slices.Clone(lines[i+1 : end]),
		})
		i = end
	}
	return file
}

// find 定位接口的所有段落，无法安全改写时返回原因
func (b *ifupdownBackend) find(files []interfacesFile, name string) (*interfacesFile, []interfacesStanza, string) {
	var file *interfacesFile
	var stanzas []interfacesStanza
	for i := range files {
		for _, line := range files[i].lines {
			if fields := strings.Fields(line); len(fields) > 1 && fields[0] == "mapping" {
				return nil, nil, "ifupdown mapping configuration cannot be edited safely"
			}
		}
		for _, stanza := range files[i].stanzas {
			if stanza.name != name {
				continue
			}
			if file != nil && file.path != files[i].path {
				return nil, nil, "ifupdown interface is defined in multiple files"
			}
			if stanza.value("inherits") != "" {
				return nil, nil, "ifupdown inheritance cannot be edited safely"
			}
			file = &files[i]
			stanzas = append(stanzas, stanza)
		}
	}
	if len(stanzas) == 0 {
		return nil, nil, "no matching ifupdown interface definition"
	}
	// 段落之间夹着其他接口时无法整段替换
	for _, stanza := range file.stanzas {
		if stanza.name != name && stanza.start > stanzas[0].start && stanza.start < stanzas[len(stanzas)-1].end {
			return nil, nil, "ifupdown interface has interleaved definitions"
		}
	}
	return file, stanzas, ""
}

func (b *ifupdownBackend) family(stanzas []interfacesStanza, family string) FamilyConfig {
	config := emptyFamily()
	for _, stanza := range stanzas {
		if stanza.family != family {
			continue
		}
		switch {
		case stanza.method == "dhcp" || stanza.method == "auto":
			config.Mode = modeAuto
		case stanza.method == "static" && config.Mode != modeAuto:
			config.Mode = modeManual
		}
		if address := stanza.address(); address != "" {
			config.Addresses = append(config.Addresses, address)
		}
		if config.Gateway == "" {
			config.Gateway = stanza.value("gateway")
		}
		config.DNS = append(config.DNS, strings.Fields(stanza.value("dns-nameservers"))...)
		if family == "inet6" && config.Mode == modeAuto {
			config.AutoDNS = stanza.value("dhcp") != "0"
		}
	}
	config.Addresses, config.DNS = unique(config.Addresses), unique(config.DNS)
	return config
}

func (b *ifupdownBackend) render(config Config, unmanaged4, unmanaged6 []string) []string {
	lines := b.renderFamily(config.Name, "inet", config.IPv4, config.MTU, unmanaged4)
	lines = append(lines, "\n")
	return append(lines, b.renderFamily(config.Name, "inet6", config.IPv6, config.MTU, unmanaged6)...)
}

func (b *ifupdownBackend) renderFamily(name, family string, config FamilyConfig, mtu int, unmanaged []string) []string {
	method := "manual"
	switch config.Mode {
	case modeAuto:
		method = "dhcp"
		if family == "inet6" {
			method = "auto"
		}
	case modeManual:
		method = "static"
	}

	lines := []string{fmt.Sprintf("iface %s %s %s\n", name, family, method)}
	addresses := config.Addresses
	// 一个段落只能声明一个地址，多余的地址各起一个 static 段落
	if config.Mode == modeManual && len(addresses) > 0 {
		lines = append(lines, "    address "+addresses[0]+"\n")
		addresses = addresses[1:]
	}
	if config.Gateway != "" {
		lines = append(lines, "    gateway "+config.Gateway+"\n")
	}
	if len(config.DNS) > 0 {
		lines = append(lines, "    dns-nameservers "+strings.Join(config.DNS, " ")+"\n")
	}
	if family == "inet6" {
		switch config.Mode {
		case modeAuto:
			dhcp := "0"
			if config.AutoDNS {
				dhcp = "1"
			}
			lines = append(lines, "    accept_ra 2\n", "    dhcp "+dhcp+"\n")
		case modeDisabled:
			lines = append(lines, "    accept_ra 0\n", "    autoconf 0\n")
		}
	}
	if mtu > 0 {
		lines = append(lines, fmt.Sprintf("    mtu %d\n", mtu))
	}
	lines = append(lines, unmanaged...)

	for _, address := range addresses {
		lines = append(lines, "\n", fmt.Sprintf("iface %s %s static\n", name, family), "    address "+address+"\n")
	}
	return lines
}

// unmanaged 提取面板不生成的选项，改写时原样保留
func (b *ifupdownBackend) unmanaged(stanzas []interfacesStanza, family string) []string {
	// managed 由面板生成的选项，其余选项原样保留
	managed := []string{"address", "netmask", "gateway", "dns-nameservers", "mtu", "accept_ra", "autoconf", "dhcp"}

	var result []string
	for _, stanza := range stanzas {
		if stanza.family != family {
			continue
		}
		for _, line := range stanza.options {
			if fields := strings.Fields(line); len(fields) > 0 && !slices.Contains(managed, fields[0]) {
				result = append(result, line)
			}
		}
	}
	return result
}

// write 先在同目录落临时文件并用 ifquery 校验语法，通过后原子替换
func (b *ifupdownBackend) write(ctx context.Context, path, name, content string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".acepanel-interfaces-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err = tmp.WriteString(content); err == nil {
		err = tmp.Chmod(0644)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if _, err = run(ctx, "ifquery", "--interfaces", tmp.Name(), name); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func (b *ifupdownBackend) reload(ctx context.Context, name string) error {
	if hasCommand("ifreload") {
		_, err := run(ctx, "ifreload", "-c")
		return err
	}
	if _, err := run(ctx, "ifdown", "--force", name); err != nil {
		return err
	}
	_, err := run(ctx, "ifup", name)
	return err
}

func (s interfacesStanza) value(key string) string {
	for _, line := range s.options {
		if fields := strings.Fields(line); len(fields) > 1 && fields[0] == key {
			return strings.Join(fields[1:], " ")
		}
	}
	return ""
}

// address 把 address 与 netmask 合并为 CIDR
func (s interfacesStanza) address() string {
	address, mask := s.value("address"), s.value("netmask")
	if address == "" || mask == "" || strings.Contains(address, "/") {
		return address
	}
	if bits, err := strconv.Atoi(mask); err == nil {
		return address + "/" + strconv.Itoa(bits)
	}
	if parsed := net.ParseIP(mask); parsed != nil {
		if bits, _ := net.IPMask(parsed.To4()).Size(); bits > 0 {
			return address + "/" + strconv.Itoa(bits)
		}
	}
	return address
}
