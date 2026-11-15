package service

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	stdos "os"
	"slices"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
)

// passwordEnv 交互输入前回退读取的密码环境变量
const passwordEnv = "ACEPANEL_PASSWORD"

func (s *CliService) UserList(ctx context.Context, cmd *cli.Command) error {
	users, _, err := s.userRepo.List(ctx, 1, math.MaxUint32)
	if err != nil {
		return errors.New(s.t.Get("Failed to get user list: %v", err))
	}
	slices.Reverse(users)

	return s.printList(cmd, users, func() {
		for _, user := range users {
			fmt.Println(s.t.Get("ID: %d, Username: %s, Email: %s, Created At: %s", user.ID, user.Username, user.Email, user.CreatedAt.Format(time.DateTime)))
		}
	})
}

func (s *CliService) UserCreate(ctx context.Context, cmd *cli.Command) error {
	username := cmd.StringArg("username")
	if username == "" {
		return errors.New(s.t.Get("Username cannot be empty"))
	}

	password, err := s.readSecret(cmd.StringArg("password"), passwordEnv, s.t.Get("Please enter the password: "))
	if err != nil {
		return err
	}
	if len(password) < 6 {
		return errors.New(s.t.Get("Password length cannot be less than 6"))
	}

	// 邮箱仅用于占位，面板不强制校验可达性
	email := cmd.String("email")
	if email == "" {
		email = username + "@example.com"
	}

	user, err := s.userRepo.Create(ctx, username, password, email)
	if err != nil {
		return errors.New(s.t.Get("Failed to create user: %v", err))
	}

	fmt.Println(s.t.Get("User %s created successfully", user.Username))
	return nil
}

func (s *CliService) UserDelete(ctx context.Context, cmd *cli.Command) error {
	username := cmd.StringArg("username")
	if username == "" {
		return errors.New(s.t.Get("Username cannot be empty"))
	}

	user, err := s.getUser(username)
	if err != nil {
		return err
	}

	if err = s.userRepo.Delete(ctx, user.ID); err != nil {
		return errors.New(s.t.Get("Failed to delete user: %v", err))
	}

	fmt.Println(s.t.Get("User %s deleted successfully", username))
	return nil
}

func (s *CliService) UserName(ctx context.Context, cmd *cli.Command) error {
	oldUsername := cmd.StringArg("old-username")
	newUsername := cmd.StringArg("new-username")
	if oldUsername == "" {
		return errors.New(s.t.Get("Old username cannot be empty"))
	}
	if newUsername == "" {
		return errors.New(s.t.Get("New username cannot be empty"))
	}

	user, err := s.getUser(oldUsername)
	if err != nil {
		return err
	}

	if err = s.userRepo.UpdateUsername(ctx, user.ID, newUsername); err != nil {
		return errors.New(s.t.Get("Failed to change username: %v", err))
	}

	fmt.Println(s.t.Get("Username %s changed to %s successfully", oldUsername, newUsername))
	return nil
}

func (s *CliService) UserPassword(ctx context.Context, cmd *cli.Command) error {
	username := cmd.StringArg("username")
	if username == "" {
		return errors.New(s.t.Get("Username cannot be empty"))
	}

	password, err := s.readSecret(cmd.StringArg("password"), passwordEnv, s.t.Get("Please enter the new password: "))
	if err != nil {
		return err
	}
	if len(password) < 6 {
		return errors.New(s.t.Get("Password length cannot be less than 6"))
	}

	user, err := s.getUser(username)
	if err != nil {
		return err
	}

	if err = s.userRepo.UpdatePassword(ctx, user.ID, password); err != nil {
		return errors.New(s.t.Get("Failed to change password: %v", err))
	}

	fmt.Println(s.t.Get("Password for user %s changed successfully", username))
	return nil
}

func (s *CliService) UserTwoFA(ctx context.Context, cmd *cli.Command) error {
	username := cmd.StringArg("username")
	if username == "" {
		return errors.New(s.t.Get("Username cannot be empty"))
	}

	user, err := s.getUser(username)
	if err != nil {
		return err
	}

	// 已开启，关闭2FA
	if user.TwoFA != "" {
		if err = s.userRepo.UpdateTwoFA(ctx, user.ID, "", ""); err != nil {
			return errors.New(s.t.Get("Failed to change 2FA status: %v", err))
		}
		fmt.Println(s.t.Get("2FA disabled for user %s", username))
		return nil
	}
	// 未开启，开启2FA
	_, url, secret, err := s.userRepo.GenerateTwoFA(user.ID)
	if err != nil {
		return errors.New(s.t.Get("Failed to generate 2FA: %v", err))
	}
	fmt.Println(s.t.Get("2FA url: %s", url))
	reader := bufio.NewReader(stdos.Stdin)
	fmt.Print(s.t.Get("Please enter the 2FA code: "))
	code, err := reader.ReadString('\n')
	if err != nil {
		return errors.New(s.t.Get("Failed to read input: %v", err))
	}
	if err = s.userRepo.UpdateTwoFA(ctx, user.ID, strings.TrimSpace(code), secret); err != nil {
		return errors.New(s.t.Get("Failed to update 2FA: %v", err))
	}

	return nil
}

func (s *CliService) UserPasskey(ctx context.Context, cmd *cli.Command) error {
	username := cmd.StringArg("username")
	if username == "" {
		return errors.New(s.t.Get("Username cannot be empty"))
	}

	user, err := s.getUser(username)
	if err != nil {
		return err
	}

	if err = s.userPasskeyRepo.DeleteAllByUserID(user.ID); err != nil {
		return errors.New(s.t.Get("Failed to clear passkeys: %v", err))
	}
	s.log.Warn(s.t.Get("user passkeys cleared"), slog.String("type", biz.OperationTypeUser), slog.Uint64("operator_id", biz.OperatorID(ctx)), slog.Uint64("id", uint64(user.ID)), slog.String("username", user.Username))

	fmt.Println(s.t.Get("All passkeys cleared for user %s", username))
	return nil
}

// getUser 按用户名取用户，不存在时给出明确提示
func (s *CliService) getUser(username string) (*biz.User, error) {
	user, err := s.userRepo.GetByUsername(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(s.t.Get("User not exists"))
		}
		return nil, errors.New(s.t.Get("Failed to get user: %v", err))
	}

	return user, nil
}
