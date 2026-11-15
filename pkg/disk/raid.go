package disk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/acepanel/panel/v3/pkg/shell"
)

// RAID 探测到的 RAID 配置
type RAID struct {
	Type        string           `json:"type"` // mdadm、megaraid、hpsa、adaptec
	Controllers []RAIDController `json:"controllers"`
	Arrays      []RAIDArray      `json:"arrays"`
}

// RAIDController RAID 控制器信息
type RAIDController struct {
	Model    string `json:"model"`
	Serial   string `json:"serial"`
	Firmware string `json:"firmware"`
	Cache    string `json:"cache_size"`
}

// RAIDArray RAID 阵列信息
type RAIDArray struct {
	Name          string       `json:"name"`
	RaidLevel     string       `json:"raid_level"`
	Size          string       `json:"size"`
	State         string       `json:"state"`
	StripSize     string       `json:"strip_size"`
	ActiveDevices int          `json:"active_devices"`
	TotalDevices  int          `json:"total_devices"`
	RebuildPct    string       `json:"rebuild_pct,omitempty"`
	Devices       []RAIDDevice `json:"devices"`
}

// RAIDDevice RAID 物理磁盘信息
type RAIDDevice struct {
	Name   string `json:"name"`
	Slot   string `json:"slot"`
	Size   string `json:"size"`
	State  string `json:"state"`
	Model  string `json:"model"`
	Serial string `json:"serial"`
}

// DetectRAID 依次探测软件 RAID (mdadm)、MegaRAID (LSI/Broadcom)、HP Smart Array、Adaptec，
// 返回首个命中的配置，都没有时返回 nil
func DetectRAID(ctx context.Context) *RAID {
	for _, detect := range []func(context.Context) *RAID{detectMdadm, detectMegaRAID, detectHPSA, detectAdaptec} {
		if raid := detect(ctx); raid != nil {
			return raid
		}
	}

	return nil
}

func detectMdadm(ctx context.Context) *RAID {
	mdstat, err := os.ReadFile("/proc/mdstat")
	if err != nil {
		return nil
	}
	names := parseMdstat(string(mdstat))
	if len(names) == 0 {
		return nil
	}

	var arrays []RAIDArray
	for _, name := range names {
		detail, _ := shell.ExecfWithTimeout(ctx, 10*time.Second, "mdadm --detail '/dev/%s' 2>/dev/null", name)
		if detail == "" {
			continue
		}
		arrays = append(arrays, parseMdadm(name, detail))
	}

	return &RAID{
		Type:        "mdadm",
		Controllers: []RAIDController{},
		Arrays:      arrays,
	}
}

// parseMdstat 提取 /proc/mdstat 中的阵列名
func parseMdstat(mdstat string) []string {
	var names []string
	for line := range strings.SplitSeq(mdstat, "\n") {
		// 阵列行形如 "md0 : active raid1 sdb1[1] sda1[0]"，首行的 Personalities 也是同样的格式
		if name, _, ok := strings.Cut(strings.TrimSpace(line), " : "); ok && name != "Personalities" {
			names = append(names, name)
		}
	}

	return names
}

// parseMdadm 解析 mdadm --detail 输出
func parseMdadm(name, detail string) RAIDArray {
	arr := RAIDArray{Name: name}

	// 磁盘列表在 Number Major Minor RaidDevice State 表头之后
	inDevSection := false
	for line := range strings.SplitSeq(detail, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Number") && strings.Contains(line, "RaidDevice") {
			inDevSection = true
			continue
		}

		if inDevSection {
			// 状态可能是多个词（active sync、spare rebuilding），设备路径在最后；removed 的槽位没有设备路径，只有 5 个字段
			if fields := strings.Fields(line); len(fields) >= 6 {
				arr.Devices = append(arr.Devices, RAIDDevice{
					Name:  fields[len(fields)-1],
					Slot:  fields[3],
					State: strings.Join(fields[4:len(fields)-1], " "),
				})
			}
			continue
		}

		if after, ok := strings.CutPrefix(line, "Raid Level :"); ok {
			arr.RaidLevel = strings.TrimSpace(after)
		} else if after, ok := strings.CutPrefix(line, "Array Size :"); ok {
			arr.Size = strings.TrimSpace(after)
		} else if after, ok := strings.CutPrefix(line, "State :"); ok {
			arr.State = strings.TrimSpace(after)
		} else if after, ok := strings.CutPrefix(line, "Active Devices :"); ok {
			_, _ = fmt.Sscanf(after, "%d", &arr.ActiveDevices)
		} else if after, ok := strings.CutPrefix(line, "Total Devices :"); ok {
			_, _ = fmt.Sscanf(after, "%d", &arr.TotalDevices)
		} else if after, ok := strings.CutPrefix(line, "Chunk Size :"); ok {
			arr.StripSize = strings.TrimSpace(after)
		} else if after, ok := strings.CutPrefix(line, "Rebuild Status :"); ok {
			arr.RebuildPct = strings.TrimSpace(after)
		}
	}

	return arr
}

func detectMegaRAID(ctx context.Context) *RAID {
	storcli := lookPath("storcli64", "storcli")
	if storcli == "" {
		return nil
	}

	output, err := shell.ExecfWithTimeout(ctx, 30*time.Second, "%s /cALL show all J", storcli)
	if err != nil || output == "" {
		return nil
	}

	controllers, arrays := parseMegaRAID(output)
	if len(controllers) == 0 && len(arrays) == 0 {
		return nil
	}

	return &RAID{
		Type:        "megaraid",
		Controllers: controllers,
		Arrays:      arrays,
	}
}

// parseMegaRAID 解析 storcli JSON 输出
func parseMegaRAID(output string) ([]RAIDController, []RAIDArray) {
	var data map[string]any
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return nil, nil
	}

	var controllers []RAIDController
	var arrays []RAIDArray

	// storcli JSON 结构: Controllers[].Response Data
	ctrlList, _ := data["Controllers"].([]any)
	for _, ctrl := range ctrlList {
		ctrlMap, _ := ctrl.(map[string]any)
		respData, _ := ctrlMap["Response Data"].(map[string]any)
		if respData == nil {
			continue
		}

		// 控制器基本信息
		if basics, ok := respData["Basics"].(map[string]any); ok {
			controllers = append(controllers, RAIDController{
				Model:    fmt.Sprintf("%v", basics["Model"]),
				Serial:   fmt.Sprintf("%v", basics["Serial Number"]),
				Firmware: fmt.Sprintf("%v", basics["FW Package Build"]),
			})
		}

		// 虚拟磁盘（阵列）
		if vdList, ok := respData["VD LIST"].([]any); ok {
			for _, vd := range vdList {
				vdMap, _ := vd.(map[string]any)
				if vdMap == nil {
					continue
				}
				arrays = append(arrays, RAIDArray{
					Name:      fmt.Sprintf("%v", vdMap["DG/VD"]),
					RaidLevel: fmt.Sprintf("%v", vdMap["TYPE"]),
					Size:      fmt.Sprintf("%v", vdMap["Size"]),
					State:     fmt.Sprintf("%v", vdMap["State"]),
				})
			}
		}

		// 物理磁盘
		if pdList, ok := respData["PD LIST"].([]any); ok {
			for _, pd := range pdList {
				pdMap, _ := pd.(map[string]any)
				if pdMap == nil {
					continue
				}
				dev := RAIDDevice{
					Slot:   fmt.Sprintf("%v", pdMap["EID:Slt"]),
					Size:   fmt.Sprintf("%v", pdMap["Size"]),
					State:  fmt.Sprintf("%v", pdMap["State"]),
					Model:  fmt.Sprintf("%v", pdMap["Model"]),
					Serial: fmt.Sprintf("%v", pdMap["SN"]),
				}
				// 将物理磁盘分配到对应的阵列
				if dg, ok := pdMap["DG"].(float64); ok && int(dg) < len(arrays) {
					arrays[int(dg)].Devices = append(arrays[int(dg)].Devices, dev)
				}
			}
		}
	}

	return controllers, arrays
}

func detectHPSA(ctx context.Context) *RAID {
	ssacli := lookPath("ssacli", "hpssacli")
	if ssacli == "" {
		return nil
	}

	output, err := shell.ExecfWithTimeout(ctx, 30*time.Second, "%s ctrl all show config detail", ssacli)
	if err != nil || output == "" {
		return nil
	}

	controllers, arrays := parseHPSA(output)
	if len(controllers) == 0 && len(arrays) == 0 {
		return nil
	}

	return &RAID{
		Type:        "hpsa",
		Controllers: controllers,
		Arrays:      arrays,
	}
}

// parseHPSA 解析 ssacli 文本输出
func parseHPSA(output string) ([]RAIDController, []RAIDArray) {
	var controllers []RAIDController
	var arrays []RAIDArray

	var currentCtrl *RAIDController
	var currentArray *RAIDArray
	var currentDev *RAIDDevice

	for line := range strings.SplitSeq(output, "\n") {
		trimmed := strings.TrimSpace(line)

		// 控制器
		if strings.Contains(trimmed, "Smart Array") || strings.Contains(trimmed, "Smart HBA") {
			if currentCtrl != nil {
				controllers = append(controllers, *currentCtrl)
			}
			currentCtrl = &RAIDController{Model: trimmed}
		}

		if currentCtrl != nil {
			if after, ok := strings.CutPrefix(trimmed, "Serial Number:"); ok {
				currentCtrl.Serial = strings.TrimSpace(after)
			} else if after, ok := strings.CutPrefix(trimmed, "Firmware Version:"); ok {
				currentCtrl.Firmware = strings.TrimSpace(after)
			} else if strings.HasPrefix(trimmed, "Cache Board Present:") || strings.HasPrefix(trimmed, "Total Cache Size:") {
				currentCtrl.Cache = colonValue(trimmed)
			}
		}

		// 阵列
		if strings.HasPrefix(trimmed, "Array:") || (strings.HasPrefix(trimmed, "array") && strings.Contains(trimmed, "Array")) {
			if currentArray != nil {
				if currentDev != nil {
					currentArray.Devices = append(currentArray.Devices, *currentDev)
					currentDev = nil
				}
				arrays = append(arrays, *currentArray)
			}
			currentArray = &RAIDArray{Name: trimmed}
		}

		if currentArray != nil {
			if after, ok := strings.CutPrefix(trimmed, "Fault Tolerance:"); ok {
				currentArray.RaidLevel = strings.TrimSpace(after)
			} else if after, ok := strings.CutPrefix(trimmed, "Size:"); ok {
				currentArray.Size = strings.TrimSpace(after)
			} else if after, ok := strings.CutPrefix(trimmed, "Status:"); ok {
				currentArray.State = strings.TrimSpace(after)
			} else if after, ok := strings.CutPrefix(trimmed, "Strip Size:"); ok {
				currentArray.StripSize = strings.TrimSpace(after)
			}

			// 物理磁盘
			if strings.HasPrefix(trimmed, "physicaldrive") {
				if currentDev != nil {
					currentArray.Devices = append(currentArray.Devices, *currentDev)
				}
				currentDev = &RAIDDevice{Name: trimmed}
			}
			if currentDev != nil {
				if strings.HasPrefix(trimmed, "Port:") || strings.HasPrefix(trimmed, "Bay:") {
					currentDev.Slot = colonValue(trimmed)
				} else if after, ok := strings.CutPrefix(trimmed, "Size:"); ok {
					currentDev.Size = strings.TrimSpace(after)
				} else if after, ok := strings.CutPrefix(trimmed, "Status:"); ok {
					currentDev.State = strings.TrimSpace(after)
				} else if after, ok := strings.CutPrefix(trimmed, "Model:"); ok {
					currentDev.Model = strings.TrimSpace(after)
				} else if after, ok := strings.CutPrefix(trimmed, "Serial Number:"); ok {
					currentDev.Serial = strings.TrimSpace(after)
				}
			}
		}
	}

	// 收尾
	if currentDev != nil && currentArray != nil {
		currentArray.Devices = append(currentArray.Devices, *currentDev)
	}
	if currentArray != nil {
		arrays = append(arrays, *currentArray)
	}
	if currentCtrl != nil {
		controllers = append(controllers, *currentCtrl)
	}

	return controllers, arrays
}

func detectAdaptec(ctx context.Context) *RAID {
	if lookPath("arcconf") == "" {
		return nil
	}

	output, err := shell.ExecfWithTimeout(ctx, 30*time.Second, "arcconf GETCONFIG 1")
	if err != nil || output == "" {
		return nil
	}

	controllers, arrays := parseAdaptec(output)
	if len(controllers) == 0 && len(arrays) == 0 {
		return nil
	}

	return &RAID{
		Type:        "adaptec",
		Controllers: controllers,
		Arrays:      arrays,
	}
}

// parseAdaptec 解析 arcconf GETCONFIG 输出
func parseAdaptec(output string) ([]RAIDController, []RAIDArray) {
	var controllers []RAIDController
	var arrays []RAIDArray

	var currentCtrl *RAIDController
	var currentArray *RAIDArray
	var currentDev *RAIDDevice
	inLogicalDev := false
	inPhysicalDev := false

	for line := range strings.SplitSeq(output, "\n") {
		trimmed := strings.TrimSpace(line)

		// 控制器信息
		if strings.Contains(trimmed, "Controller Model") {
			currentCtrl = &RAIDController{Model: colonValue(trimmed)}
		}
		if currentCtrl != nil {
			if strings.Contains(trimmed, "Controller Serial Number") {
				currentCtrl.Serial = colonValue(trimmed)
			} else if strings.Contains(trimmed, "Firmware") && strings.Contains(trimmed, "Version") {
				currentCtrl.Firmware = colonValue(trimmed)
			}
		}

		// 逻辑设备段
		if strings.HasPrefix(trimmed, "Logical Device number") || strings.HasPrefix(trimmed, "Logical device number") {
			if currentArray != nil {
				if currentDev != nil {
					currentArray.Devices = append(currentArray.Devices, *currentDev)
					currentDev = nil
				}
				arrays = append(arrays, *currentArray)
			}
			currentArray = &RAIDArray{Name: trimmed}
			inLogicalDev = true
			inPhysicalDev = false
			continue
		}

		if inLogicalDev && currentArray != nil {
			switch {
			case strings.HasPrefix(trimmed, "RAID level"):
				currentArray.RaidLevel = colonValue(trimmed)
			case strings.HasPrefix(trimmed, "Size"):
				currentArray.Size = colonValue(trimmed)
			case strings.HasPrefix(trimmed, "Status of Logical Device"), strings.HasPrefix(trimmed, "Status of logical device"):
				currentArray.State = colonValue(trimmed)
			case strings.HasPrefix(trimmed, "Stripe-size"), strings.HasPrefix(trimmed, "Strip Size"):
				currentArray.StripSize = colonValue(trimmed)
			}
		}

		// 物理设备段
		if strings.Contains(trimmed, "Device #") || strings.HasPrefix(trimmed, "Physical Device") {
			if currentDev != nil && currentArray != nil {
				currentArray.Devices = append(currentArray.Devices, *currentDev)
			}
			currentDev = &RAIDDevice{Name: trimmed}
			inPhysicalDev = true
		}

		if inPhysicalDev && currentDev != nil {
			switch {
			case strings.HasPrefix(trimmed, "State"):
				currentDev.State = colonValue(trimmed)
			case strings.HasPrefix(trimmed, "Size"):
				currentDev.Size = colonValue(trimmed)
			case strings.HasPrefix(trimmed, "Model"):
				currentDev.Model = colonValue(trimmed)
			case strings.HasPrefix(trimmed, "Serial number"), strings.HasPrefix(trimmed, "Serial Number"):
				currentDev.Serial = colonValue(trimmed)
			case strings.HasPrefix(trimmed, "Reported Channel,Device"):
				currentDev.Slot = colonValue(trimmed)
			}
		}
	}

	// 收尾
	if currentDev != nil && currentArray != nil {
		currentArray.Devices = append(currentArray.Devices, *currentDev)
	}
	if currentArray != nil {
		arrays = append(arrays, *currentArray)
	}
	if currentCtrl != nil {
		controllers = append(controllers, *currentCtrl)
	}

	return controllers, arrays
}

// lookPath 返回 names 中首个已安装的命令，都没有时返回空串
func lookPath(names ...string) string {
	for _, name := range names {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}

	return ""
}

// colonValue 从 "Key : Value" 格式中提取值
func colonValue(line string) string {
	_, value, _ := strings.Cut(line, ":")
	return strings.TrimSpace(value)
}
