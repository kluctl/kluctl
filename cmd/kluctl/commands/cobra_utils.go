package commands

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/kluctl/kluctl/v2/lib/envutils"
	"github.com/kluctl/kluctl/v2/pkg/utils/term"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type helpProvider interface {
	Help() string
}

type runProvider interface {
	Run(ctx context.Context) error
}

type rootCommand struct {
	rootCmd    *commandAndGroups
	groupInfos []groupInfo
}

type commandAndGroups struct {
	parent *commandAndGroups
	cmd    *cobra.Command
	groups map[string]string
}

type groupInfo struct {
	group       string
	title       string
	description string
}

func buildRootCobraCmd(cmdStruct interface{}, name string, short string, long string, groupInfos []groupInfo) (*cobra.Command, error) {
	c := &rootCommand{
		groupInfos: groupInfos,
	}

	cmd, err := c.buildCobraCmd(nil, cmdStruct, name, short, long)
	if err != nil {
		return nil, err
	}
	c.rootCmd = cmd

	return cmd.cmd, nil
}

func (c *rootCommand) buildCobraCmd(parent *commandAndGroups, cmdStruct interface{}, name string, short string, long string) (*commandAndGroups, error) {
	cg := &commandAndGroups{
		parent: parent,
		groups: map[string]string{},
		cmd: &cobra.Command{
			Use:   name,
			Short: short,
			Long:  long,
		},
	}

	runP, ok := cmdStruct.(runProvider)
	if ok {
		cg.cmd.RunE = func(cmd *cobra.Command, args []string) error {
			return runP.Run(cmd.Context())
		}
	}

	err := c.buildCobraSubCommands(cg, cmdStruct)
	if err != nil {
		return nil, err
	}
	err = c.buildCobraArgs(cg, cmdStruct, "")
	if err != nil {
		return nil, err
	}

	err = RegisterFlagCompletionFuncs(cmdStruct, cg.cmd)
	if err != nil {
		return nil, err
	}

	cg.cmd.SetHelpFunc(func(command *cobra.Command, i []string) {
		c.helpFunc(cg, command)
	})

	return cg, nil
}

func (c *rootCommand) buildCobraSubCommands(cg *commandAndGroups, cmdStruct interface{}) error {
	v := reflect.ValueOf(cmdStruct).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		v2 := v.Field(i).Addr().Interface()
		name := buildCobraName(f.Name)

		if cmd, ok := f.Tag.Lookup("cmd"); ok {
			if cmd != "" {
				name = cmd
			}
			// subcommand
			shortHelp := f.Tag.Get("help")
			longHelp := ""
			if hp, ok := v2.(helpProvider); ok {
				longHelp = hp.Help()
			}

			c2, err := c.buildCobraCmd(cg, v2, name, shortHelp, longHelp)
			if err != nil {
				return err
			}
			cg.cmd.AddCommand(c2.cmd)
		}
	}
	return nil
}

func (c *rootCommand) buildCobraArgs(cg *commandAndGroups, cmdStruct interface{}, groupOverride string) error {
	v := reflect.ValueOf(cmdStruct).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		if _, ok := f.Tag.Lookup("cmd"); ok {
			continue
		}

		groupOverride2, _ := f.Tag.Lookup("groupOverride")
		if groupOverride2 == "" {
			groupOverride2 = groupOverride
		}

		err := c.buildCobraArg(cg, f, v.Field(i), groupOverride2)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *rootCommand) buildCobraArg(cg *commandAndGroups, f reflect.StructField, v reflect.Value, groupOverride string) error {
	v2 := v.Addr().Interface()
	name := buildCobraName(f.Name)

	help := f.Tag.Get("help")
	shortFlag := f.Tag.Get("short")
	defaultValue := f.Tag.Get("default")
	required := f.Tag.Get("required") == "true"
	skipEnv := f.Tag.Get("skipenv")

	group := groupOverride
	if group == "" {
		group = f.Tag.Get("group")
	}
	if group != "" {
		cg.groups[name] = group
	}

	switch v2.(type) {
	case pflag.Value:
		v3 := v2.(pflag.Value)
		cg.cmd.PersistentFlags().VarP(v3, name, shortFlag, help)
		switch v3.Type() {
		case "existingfile":
			exts := strings.Split(f.Tag.Get("exts"), ",")
			_ = cg.cmd.MarkPersistentFlagFilename(name, exts...)
		case "existingdir":
			_ = cg.cmd.MarkPersistentFlagDirname(name)
		case "existingpath":
			exts := strings.Split(f.Tag.Get("exts"), ",")
			_ = cg.cmd.MarkPersistentFlagFilename(name, exts...)
			_ = cg.cmd.MarkPersistentFlagDirname(name)
		}
	case *string:
		cg.cmd.PersistentFlags().StringVarP(v2.(*string), name, shortFlag, defaultValue, help)
	case *[]string:
		if defaultValue != "" {
			return fmt.Errorf("default not supported for slices")
		}
		cg.cmd.PersistentFlags().StringArrayVarP(v2.(*[]string), name, shortFlag, nil, help)
	case *bool:
		parsedDefault := false
		if defaultValue != "" {
			x, err := strconv.ParseBool(defaultValue)
			if err != nil {
				return err
			}
			parsedDefault = x
		}
		cg.cmd.PersistentFlags().BoolVarP(v2.(*bool), name, shortFlag, parsedDefault, help)
	case *int:
		parsedDefault := 0
		if defaultValue != "" {
			x, err := strconv.ParseInt(defaultValue, 0, 32)
			if err != nil {
				return err
			}
			parsedDefault = int(x)
		}
		cg.cmd.PersistentFlags().IntVarP(v2.(*int), name, shortFlag, parsedDefault, help)
	case *time.Duration:
		var parsedDefault time.Duration
		if defaultValue != "" {
			x, err := time.ParseDuration(defaultValue)
			if err != nil {
				return err
			}
			parsedDefault = x
		}
		cg.cmd.PersistentFlags().DurationVarP(v2.(*time.Duration), name, shortFlag, parsedDefault, help)
	default:
		if f.Anonymous {
			return c.buildCobraArgs(cg, v2, groupOverride)
		}
		return fmt.Errorf("unknown type %s", f.Type.Name())
	}

	if required {
		_ = cg.cmd.MarkPersistentFlagRequired(name)
	}

	_ = cg.cmd.PersistentFlags().SetAnnotation(name, "skipenv", []string{skipEnv})

	return nil
}

func copyViperValuesToCobraCmd(cmd *cobra.Command) error {
	for cmd != nil {
		err := copyViperValuesToCobraFlags(cmd.PersistentFlags())
		if err != nil {
			return err
		}
		cmd = cmd.Parent()
	}
	return nil
}

func copyViperValuesToCobraFlags(flags *pflag.FlagSet) error {
	var errs *multierror.Error
	flags.VisitAll(func(flag *pflag.Flag) {
		if a := flag.Annotations["skipenv"]; len(a) != 0 && a[0] == "true" {
			return
		}

		sliceValue, _ := flag.Value.(pflag.SliceValue)
		if flag.Changed && sliceValue == nil {
			return
		}
		v := viper.Get(flag.Name)

		var a []string
		if v != nil {
			if x, ok := v.(string); ok {
				a = []string{x}
			} else if x, ok := v.(bool); ok {
				a = []string{strconv.FormatBool(x)}
			} else if x, ok := v.(int); ok {
				a = []string{strconv.FormatInt(int64(x), 10)}
			} else if x, ok := v.([]any); ok {
				for _, y := range x {
					s, ok := y.(string)
					if !ok {
						errs = multierror.Append(errs, fmt.Errorf("viper flag %s has unexpected type", flag.Name))
						return
					}
					a = append(a, s)
				}
			} else {
				errs = multierror.Append(errs, fmt.Errorf("viper flag %s has unexpected type", flag.Name))
				return
			}
		}

		envName := strings.ReplaceAll(flag.Name, "-", "_")
		envName = strings.ToUpper(envName)
		envName = fmt.Sprintf("KLUCTL_%s", envName)

		for _, v := range envutils.ParseEnvConfigList(envName) {
			a = append(a, v)
		}

		if sliceValue != nil {
			// we must ensure that values passed via CLI are at the end of the slice
			a = append(a, sliceValue.GetSlice()...)
			err := sliceValue.Replace(a)
			if err != nil {
				errs = multierror.Append(errs, err)
			}
		} else {
			for _, x := range a {
				err := flag.Value.Set(x)
				if err != nil {
					errs = multierror.Append(errs, err)
				}
			}
		}
	})
	return errs.ErrorOrNil()
}

func (c *rootCommand) helpFunc(cg *commandAndGroups, cmd *cobra.Command) {
	termWidth := term.GetWidth()

	h := "Usage: "
	if cmd.Runnable() {
		h += cmd.UseLine()
	}
	if cmd.HasAvailableSubCommands() {
		h += fmt.Sprintf("%s [command]", cmd.CommandPath())
	}

	h += "\n\n"
	if cmd.Short != "" {
		h += strings.TrimRightFunc(cmd.Short, unicode.IsSpace) + "\n"
	}
	if cmd.Long != "" {
		h += strings.TrimRightFunc(cmd.Long, unicode.IsSpace) + "\n"
	}

	flagsByGroups := c.buildGroupedFlagSets(cg)

	for _, g := range c.groupInfos {
		fl, ok := flagsByGroups[g.group]
		if !ok {
			continue
		}
		h += "\n"
		h += g.title + "\n"
		if g.description != "" {
			h += "  " + g.description + "\n\n"
		}
		usages := fl.FlagUsagesWrapped(termWidth)
		usages = strings.TrimRightFunc(usages, unicode.IsSpace)
		usages += "\n"
		h += usages
	}

	if cmd.HasAvailableSubCommands() {
		h += "\nCommands:\n"
		for _, subCmd := range cmd.Commands() {
			h += "  " + rpad(subCmd.Name(), subCmd.NamePadding()) + " " + subCmd.Short + "\n"
		}
		h += fmt.Sprintf("\nUse \"%s [command] --help\" for more information about a command.\n", cmd.CommandPath())
	}

	_, _ = cmd.OutOrStdout().Write([]byte(h))
}

func (c *rootCommand) buildGroupedFlagSets(cg *commandAndGroups) map[string]*pflag.FlagSet {
	flagsByGroups := make(map[string]*pflag.FlagSet)

	x := cg
	for x != nil {
		x.cmd.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
			group, ok := x.groups[flag.Name]
			if !ok {
				panic(fmt.Sprintf("group for %s not found", flag.Name))
			}
			fl, ok := flagsByGroups[group]
			if !ok {
				fl = pflag.NewFlagSet(group, pflag.PanicOnError)
				flagsByGroups[group] = fl
			}
			fl.AddFlag(flag)
		})
		x = x.parent
	}

	return flagsByGroups
}

func rpad(s string, padding int) string {
	template := fmt.Sprintf("%%-%ds", padding)
	return fmt.Sprintf(template, s)
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func buildCobraName(fieldName string) string {
	n := matchFirstCap.ReplaceAllString(fieldName, "${1}-${2}")
	n = matchAllCap.ReplaceAllString(n, "${1}-${2}")
	return strings.ToLower(n)
}
