package service

import (
	"context"
	"fmt"
	"math"

	"github.com/urfave/cli/v3"

	"github.com/acepanel/panel/v3/internal/request"
)

func (s *CliService) DatabaseAddServer(ctx context.Context, cmd *cli.Command) error {
	req := &request.DatabaseServerCreate{
		Type:     cmd.String("type"),
		Name:     cmd.String("name"),
		Host:     cmd.String("host"),
		Port:     cmd.Uint("port"),
		Username: cmd.String("username"),
		Password: cmd.String("password"),
		Remark:   cmd.String("remark"),
	}

	if err := s.validate(ctx, req); err != nil {
		return err
	}

	if err := s.databaseServerRepo.Create(ctx, req); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Database server %s added successfully", cmd.String("name")))
	return nil
}

func (s *CliService) DatabaseDeleteServer(ctx context.Context, cmd *cli.Command) error {
	server, err := s.databaseServerRepo.FindByName(cmd.String("name"))
	if err != nil {
		return err
	}

	if err = s.databaseServerRepo.Delete(ctx, server.ID); err != nil {
		return err
	}

	fmt.Println(s.t.Get("Database server %s deleted successfully", server.Name))
	return nil
}

func (s *CliService) DatabaseListServer(ctx context.Context, cmd *cli.Command) error {
	servers, _, err := s.databaseServerRepo.List(ctx, 1, math.MaxUint32, "")
	if err != nil {
		return err
	}

	return s.printList(cmd, servers, func() {
		for _, server := range servers {
			fmt.Println(s.t.Get("ID: %d, Name: %s, Type: %s, Address: %s:%d, Username: %s", server.ID, server.Name, string(server.Type), server.Host, server.Port, server.Username))
		}
	})
}
