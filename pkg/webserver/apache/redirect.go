package apache

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/acepanel/panel/pkg/webserver/types"
)

// redirectFilePattern 匹配重定向配置文件名 (100-199)
var redirectFilePattern = regexp.MustCompile(`^(\d{3})-redirect\.conf$`)

// parseRedirectFiles 从 site 目录解析所有重定向配置
func parseRedirectFiles(siteDir string) ([]types.Redirect, error) {
	entries, err := os.ReadDir(siteDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var redirects []types.Redirect
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		matches := redirectFilePattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}

		num, _ := strconv.Atoi(matches[1])
		if num < RedirectStartNum || num > RedirectEndNum {
			continue
		}

		filePath := filepath.Join(siteDir, entry.Name())
		redirect, err := parseRedirectFile(filePath)
		if err != nil {
			continue // 跳过解析失败的文件
		}
		if redirect != nil {
			redirects = append(redirects, *redirect)
		}
	}

	return redirects, nil
}

// parseRedirectFile 解析单个重定向配置文件
func parseRedirectFile(filePath string) (*types.Redirect, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	contentStr := string(content)

	// 解析 Redirect 指令: Redirect 308 /old /new
	redirectPattern := regexp.MustCompile(`Redirect\s+(\d+)\s+(\S+)\s+(\S+)`)
	if matches := redirectPattern.FindStringSubmatch(contentStr); matches != nil {
		statusCode, _ := strconv.Atoi(matches[1])
		return &types.Redirect{
			Type:       types.RedirectTypeURL,
			From:       matches[2],
			To:         matches[3],
			StatusCode: statusCode,
		}, nil
	}

	// 解析 RedirectMatch 指令: RedirectMatch 308 ^/old(.*)$ /new$1
	redirectMatchPattern := regexp.MustCompile(`RedirectMatch\s+(\d+)\s+(\S+)\s+(\S+)`)
	if matches := redirectMatchPattern.FindStringSubmatch(contentStr); matches != nil {
		statusCode, _ := strconv.Atoi(matches[1])
		to := matches[3]
		keepURI := strings.Contains(to, "$1")
		if keepURI {
			to = strings.TrimSuffix(to, "$1")
		}
		// 还原 from 为简单路径格式
		from := matches[2]
		from = strings.TrimPrefix(from, "^")
		from = strings.TrimSuffix(from, "(.*)$")
		from = strings.TrimSuffix(from, "$")
		return &types.Redirect{
			Type:       types.RedirectTypeURL,
			From:       from,
			To:         to,
			KeepURI:    keepURI,
			StatusCode: statusCode,
		}, nil
	}

	// 解析 RewriteRule Host 重定向
	// RewriteCond %{HTTP_HOST} ^old\.example\.com$
	// RewriteRule ^(.*)$ https://new.example.com$1 [R=308,L]
	hostRewritePattern := regexp.MustCompile(`RewriteCond\s+%\{HTTP_HOST}\s+\^?([^$\s]+)\$?\s*\[?NC]?\s*\n\s*RewriteRule\s+\^\(\.\*\)\$\s+([^\s\[]+)\s*\[R=(\d+)`)
	if matches := hostRewritePattern.FindStringSubmatch(contentStr); matches != nil {
		statusCode, _ := strconv.Atoi(matches[3])
		host := strings.ReplaceAll(matches[1], `\.`, ".")
		to := matches[2]
		keepURI := strings.Contains(to, "$1")
		if keepURI {
			to = strings.TrimSuffix(to, "$1")
		}
		return &types.Redirect{
			Type:       types.RedirectTypeHost,
			From:       host,
			To:         to,
			KeepURI:    keepURI,
			StatusCode: statusCode,
		}, nil
	}

	// 解析 ErrorDocument 404 重定向
	// ErrorDocument 404 /custom-404
	errorDocPattern := regexp.MustCompile(`ErrorDocument\s+404\s+(\S+)`)
	if matches := errorDocPattern.FindStringSubmatch(contentStr); matches != nil {
		return &types.Redirect{
			Type:       types.RedirectType404,
			To:         matches[1],
			StatusCode: 308,
		}, nil
	}

	return nil, nil
}

// writeRedirectFiles 将重定向配置写入文件
func writeRedirectFiles(siteDir string, redirects []types.Redirect) error {
	// 删除现有的重定向配置文件 (100-199)
	if err := clearRedirectFiles(siteDir); err != nil {
		return err
	}

	// 写入新的配置文件
	for i, redirect := range redirects {
		num := RedirectStartNum + i
		if num > RedirectEndNum {
			return fmt.Errorf("redirect rules exceed limit (%d)", RedirectEndNum-RedirectStartNum+1)
		}

		fileName := fmt.Sprintf("%03d-redirect.conf", num)
		filePath := filepath.Join(siteDir, fileName)

		content := generateRedirectConfig(redirect)
		if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
			return fmt.Errorf("failed to write redirect config: %w", err)
		}
	}

	return nil
}

// clearRedirectFiles 清除所有重定向配置文件
func clearRedirectFiles(siteDir string) error {
	entries, err := os.ReadDir(siteDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		matches := redirectFilePattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}

		num, _ := strconv.Atoi(matches[1])
		if num >= RedirectStartNum && num <= RedirectEndNum {
			filePath := filepath.Join(siteDir, entry.Name())
			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to delete redirect config: %w", err)
			}
		}
	}

	return nil
}

// generateRedirectConfig 生成重定向配置内容
func generateRedirectConfig(redirect types.Redirect) string {
	statusCode := redirect.StatusCode
	if statusCode == 0 {
		statusCode = 308 // 默认使用 308 永久重定向
	}

	var sb strings.Builder
	sb.WriteString("# Auto-generated by AcePanel. DO NOT EDIT MANUALLY!\n")

	switch redirect.Type {
	case types.RedirectTypeURL:
		// URL 重定向
		sb.WriteString(fmt.Sprintf("# URL redirect: %s -> %s\n", redirect.From, redirect.To))
		if redirect.KeepURI {
			// 使用 RedirectMatch 保持 URI
			from := redirect.From
			if !strings.HasPrefix(from, "^") {
				from = "^" + from
			}
			if !strings.HasSuffix(from, "(.*)$") && !strings.HasSuffix(from, "$") {
				from = from + "(.*)$"
			}
			to := redirect.To
			if !strings.HasSuffix(to, "$1") {
				to = to + "$1"
			}
			sb.WriteString(fmt.Sprintf("RedirectMatch %d %s %s\n", statusCode, from, to))
		} else {
			sb.WriteString(fmt.Sprintf("Redirect %d %s %s\n", statusCode, redirect.From, redirect.To))
		}

	case types.RedirectTypeHost:
		// Host 重定向
		sb.WriteString(fmt.Sprintf("# Host redirect: %s -> %s\n", redirect.From, redirect.To))
		sb.WriteString("RewriteEngine on\n")
		escapedHost := strings.ReplaceAll(redirect.From, ".", `\.`)
		sb.WriteString(fmt.Sprintf("RewriteCond %%{HTTP_HOST} ^%s$ [NC]\n", escapedHost))
		if redirect.KeepURI {
			sb.WriteString(fmt.Sprintf("RewriteRule ^(.*)$ %s$1 [R=%d,L]\n", redirect.To, statusCode))
		} else {
			sb.WriteString(fmt.Sprintf("RewriteRule ^(.*)$ %s [R=%d,L]\n", redirect.To, statusCode))
		}

	case types.RedirectType404:
		// 404 重定向
		sb.WriteString(fmt.Sprintf("# 404 redirect -> %s\n", redirect.To))
		sb.WriteString(fmt.Sprintf("ErrorDocument 404 %s\n", redirect.To))
	}

	return sb.String()
}
