package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/pkg/types"
)

func (s *CliService) CertList(ctx context.Context, cmd *cli.Command) error {
	certs, _, err := s.certRepo.List(1, math.MaxUint32)
	if err != nil {
		return err
	}

	return s.printList(cmd, certs, func() {
		for _, item := range certs {
			fmt.Println(s.t.Get("ID: %d, Domains: %s, Auto Renewal: %t, Expires At: %s", item.ID, strings.Join(item.Domains, ","), item.AutoRenewal, item.NotAfter.Format(time.DateTime)))
		}
	})
}

// CertRenew 续签证书，--all 时续签全部开启自动续签的证书
func (s *CliService) CertRenew(ctx context.Context, cmd *cli.Command) error {
	ids := []uint{}
	switch {
	case cmd.Bool("all"):
		certs, _, err := s.certRepo.List(1, math.MaxUint32)
		if err != nil {
			return err
		}
		ids = lo.FilterMap(certs, func(item *types.CertList, _ int) (uint, bool) {
			return item.ID, item.AutoRenewal
		})
	case cmd.Uint("id") != 0:
		ids = []uint{cmd.Uint("id")}
	}
	if len(ids) == 0 {
		return errors.New(s.t.Get("Please specify the certificate ID or use --all"))
	}

	var failed int
	for _, id := range ids {
		fmt.Println(s.t.Get("|-Renewing certificate %d...", id))
		if _, err := s.certRepo.RenewWithProgressCallback(ctx, id, func(msg string) {
			fmt.Println("  " + msg)
		}); err != nil {
			failed++
			fmt.Println(s.t.Get("|-Certificate %d renewal failed: %v", id, err))
			continue
		}
		fmt.Println(s.t.Get("|-Certificate %d renewed successfully", id))
	}

	if failed > 0 {
		return errors.New(s.t.Get("%d certificate(s) failed to renew", failed))
	}

	return nil
}
