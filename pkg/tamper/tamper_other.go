//go:build !linux

package tamper

import (
	"context"
	"errors"
	"log/slog"

	"github.com/leonelquinteros/gotext"
)

// Supported 非 Linux 平台不支持防篡改
func Supported() bool {
	return false
}

// DetectEBPF 非 Linux 平台不可用
func DetectEBPF() EBPFStatus {
	return EBPFStatus{Reason: "only supported on Linux"}
}

// EnableBPFLSMGrub 非 Linux 平台不支持
func EnableBPFLSMGrub(_ context.Context) error {
	return errors.New("only supported on Linux")
}

type Manager struct{}

// NewManager 非 Linux 平台不支持
func NewManager(_ Config, _ *gotext.Locale, _ *slog.Logger) (*Manager, error) {
	return nil, errors.New("tamper protection is only supported on Linux")
}

func (m *Manager) Start() error { return errors.New("not supported") }

func (m *Manager) Stop() error { return nil }

func (m *Manager) Events() <-chan Event { return nil }

func (m *Manager) Stats() Stats { return Stats{} }

func (m *Manager) Unlock(_ []string) {}

func (m *Manager) Relock(_ []string) {}
