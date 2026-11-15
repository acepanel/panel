package tui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/gookit/color"
	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"
	"github.com/urfave/cli/v3"
)

// Run 交互式驱动命令树：逐级选择命令、填写参数后交给 exec 执行，Esc 返回上一级，Ctrl+C 退出
func Run(
	ctx context.Context,
	t *gotext.Locale,
	root *cli.Command,
	exec func(context.Context, []string) error,
) error {
	fmt.Println(color.Bold.Sprint(root.Usage + " " + root.Version))
	err := newPrompt(t).loop(ctx, root, exec)

	// Esc 与 Ctrl+C 是正常退出，不当作错误
	return lo.Ternary(errors.Is(err, errBack) || errors.Is(err, errAbort), nil, err)
}

// loop 反复执行“选命令、填参数、运行、回显结果”，直到用户退出或上下文结束
func (p *prompt) loop(ctx context.Context, root *cli.Command, exec func(context.Context, []string) error) error {
	for {
		leaf, path, err := p.pickCommand(ctx, root)
		if err != nil {
			return err
		}
		args, shown, err := p.fill(ctx, leaf)
		if errors.Is(err, errBack) {
			continue
		}
		if err != nil {
			return err
		}

		prefix := append([]string{root.Name}, path...)
		fmt.Println(color.Gray.Sprint("$ " + strings.Join(slices.Concat(prefix, shown), " ")))
		if err = exec(ctx, slices.Concat(prefix, args)); err != nil {
			color.Red.Println("✘ " + p.t.Get("Failed: %v", err))
		} else {
			color.Green.Println("✔ " + p.t.Get("Done"))
		}
		fmt.Println()
		if ctx.Err() != nil {
			return errAbort
		}
	}
}

// pickCommand 逐级选择直到叶子命令，返回该命令及其名称路径
func (p *prompt) pickCommand(ctx context.Context, root *cli.Command) (*cli.Command, []string, error) {
	stack := []*cli.Command{root}
	for {
		cur := stack[len(stack)-1]
		cmds := cur.VisibleCommands()
		items := lo.Map(cmds, func(c *cli.Command, _ int) item {
			return item{Label: c.Name, Desc: c.Usage}
		})
		label, meta := p.t.Get("Command"), ""
		if cur != root {
			label, meta = cur.Name, cur.Usage
		}

		i, err := p.Select(ctx, label, meta, items, 0)
		if errors.Is(err, errBack) && len(stack) > 1 {
			stack = stack[:len(stack)-1]
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		if len(cmds[i].VisibleCommands()) > 0 {
			stack = append(stack, cmds[i])
			continue
		}

		path := lo.Map(stack[1:], func(c *cli.Command, _ int) string { return c.Name })
		return cmds[i], append(path, cmds[i].Name), nil
	}
}

// fill 先问位置参数再问标志，返回命令行参数以及密码打码后的回显文本
func (p *prompt) fill(ctx context.Context, cmd *cli.Command) ([]string, []string, error) {
	var posArgs, posShown []string
	for _, a := range cmd.Arguments {
		values, secret, err := p.askArg(ctx, a)
		if err != nil {
			return nil, nil, err
		}
		posArgs = append(posArgs, values...)
		for _, v := range values {
			posShown = append(posShown, display(v, secret))
		}
	}

	var flags, flagsShown []string
	for _, f := range cmd.VisibleFlags() {
		name := f.Names()[0]
		if name == "help" {
			continue
		}
		value, secret, err := p.askFlag(ctx, f, name)
		if err != nil {
			return nil, nil, err
		}
		if value == "" {
			continue
		}
		flags = append(flags, "--"+name+"="+value)
		flagsShown = append(flagsShown, "--"+name+"="+display(value, secret))
	}

	// 位置参数以 - 开头时加终止符，避免被当成标志
	if lo.SomeBy(posArgs, func(v string) bool { return strings.HasPrefix(v, "-") }) {
		flags, flagsShown = append(flags, "--"), append(flagsShown, "--")
	}

	return slices.Concat(flags, posArgs), slices.Concat(flagsShown, posShown), nil
}

// askArg 位置参数：用法文本以 < 开头即必填，多值参数逐个输入、留空结束
func (p *prompt) askArg(ctx context.Context, a cli.Argument) ([]string, bool, error) {
	usage := a.Usage()
	required := strings.HasPrefix(usage, "<")
	secret := a.HasName("password")
	if _, multi := a.(*cli.StringArgs); !multi {
		v, err := p.Input(ctx, usage, "", "", secret, p.notEmpty(required))
		if err != nil {
			return nil, secret, err
		}
		if v == "" {
			return nil, secret, nil
		}
		return []string{v}, secret, nil
	}

	var values []string
	for {
		v, err := p.Input(
			ctx,
			usage,
			p.t.Get("#%d, leave empty to finish", len(values)+1),
			"",
			secret,
			p.notEmpty(required && len(values) == 0),
		)
		if err != nil {
			return nil, secret, err
		}
		if v == "" {
			return values, secret, nil
		}
		values = append(values, v)
	}
}

// askFlag 按标志类型提问，返回空串表示沿用默认值
func (p *prompt) askFlag(ctx context.Context, f cli.Flag, name string) (string, bool, error) {
	meta := "--" + name
	switch fl := f.(type) {
	case *cli.BoolFlag:
		i, err := p.Select(
			ctx,
			flagLabel(fl.Usage, fl.Required),
			meta,
			[]item{{Label: p.t.Get("Yes")}, {Label: p.t.Get("No")}},
			lo.Ternary(fl.Value, 0, 1),
		)
		if err != nil || (i == 0) == fl.Value {
			return "", false, err
		}
		return strconv.FormatBool(i == 0), false, nil
	case *cli.StringFlag:
		secret := strings.Contains(name, "password")
		v, err := p.Input(
			ctx,
			flagLabel(fl.Usage, fl.Required),
			meta,
			fl.Value,
			secret,
			p.notEmpty(fl.Required),
		)
		return v, secret, err
	case *cli.StringSliceFlag:
		v, err := p.Input(
			ctx,
			flagLabel(fl.Usage, fl.Required),
			meta+" "+p.t.Get("(comma separated)"),
			strings.Join(fl.Value, ","),
			false,
			p.notEmpty(fl.Required),
		)
		return v, false, err
	case *cli.UintFlag:
		placeholder := ""
		if fl.Value != 0 {
			placeholder = strconv.FormatUint(uint64(fl.Value), 10)
		}
		v, err := p.Input(
			ctx,
			flagLabel(fl.Usage, fl.Required),
			meta,
			placeholder,
			false,
			p.integer(fl.Required, false),
		)
		return v, false, err
	case *cli.IntFlag:
		placeholder := ""
		if fl.Value != 0 {
			placeholder = strconv.Itoa(fl.Value)
		}
		v, err := p.Input(
			ctx,
			flagLabel(fl.Usage, fl.Required),
			meta,
			placeholder,
			false,
			p.integer(fl.Required, true),
		)
		return v, false, err
	}

	return "", false, nil
}

// notEmpty 必填校验
func (p *prompt) notEmpty(required bool) func(string) error {
	return func(v string) error {
		if required && v == "" {
			return errors.New(p.t.Get("This field is required"))
		}

		return nil
	}
}

// integer 十进制整数校验，留空时沿用默认值；禁止前导零，否则命令行解析时会按八进制处理
func (p *prompt) integer(required, signed bool) func(string) error {
	return func(v string) error {
		if v == "" {
			return p.notEmpty(required)(v)
		}
		digits := v
		if signed {
			digits = strings.TrimPrefix(v, "-")
		}
		nonDigit := strings.ContainsFunc(digits, func(r rune) bool { return r < '0' || r > '9' })
		if digits == "" || nonDigit || (len(digits) > 1 && digits[0] == '0') {
			return errors.New(p.t.Get("Please enter an integer"))
		}

		return nil
	}
}

// flagLabel 标志的提问文本，尖括号表示必填，方括号表示可选，与位置参数的写法一致
func flagLabel(usage string, required bool) string {
	if required {
		return "<" + usage + ">"
	}

	return "[" + usage + "]"
}

// display 回显时密码打星号，其余值按 shell 习惯加引号
func display(v string, secret bool) string {
	if secret {
		return "******"
	}
	if strings.ContainsAny(v, " \t\"'$&|;<>()*?[]!#~\\") {
		return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
	}

	return v
}
