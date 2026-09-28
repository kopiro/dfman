package dfman

import (
	"context"
	"errors"
	"fmt"
	"github.com/gofrs/flock"
	"github.com/spf13/cobra"
	"io"
	"os"
	"os/signal"
	"path/filepath"
)

type exitCode struct{ code int }

func (e exitCode) Error() string { return fmt.Sprintf("command completed with status %d", e.code) }
func Execute(args []string, version string, in io.Reader, out, errOut io.Writer) int {
	var configFile = configPath()
	var state = statePath()
	root := &cobra.Command{Use: "dfman", Short: "Manage dotfiles and their Git repositories", SilenceUsage: true, SilenceErrors: true}
	root.SetArgs(args)
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(errOut)
	root.PersistentFlags().StringVar(&configFile, "config", configFile, "TOML configuration file")
	root.PersistentFlags().StringVar(&state, "state-dir", state, "Local status directory")
	load := func() (Config, error) { return LoadConfig(configFile) }
	root.AddCommand(&cobra.Command{Use: "version", Args: cobra.NoArgs, Run: func(*cobra.Command, []string) { fmt.Fprintln(out, version) }})
	conf := &cobra.Command{Use: "config"}
	conf.AddCommand(&cobra.Command{Use: "validate", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error {
		c, e := load()
		if e != nil {
			return e
		}
		if len(c.Folders) > 0 {
			if _, e = c.Selected(""); e != nil {
				return e
			}
		}
		fmt.Fprintln(out, "Configuration is valid.")
		return nil
	}})
	root.AddCommand(conf)
	repo := &cobra.Command{Use: "repo"}
	root.AddCommand(repo)
	add := &cobra.Command{Use: "add <source> [target]", Args: cobra.RangeArgs(1, 2), RunE: func(cmd *cobra.Command, args []string) error {
		lock := flock.New(configFile + ".lock")
		if e := os.MkdirAll(filepath.Dir(configFile), 0700); e != nil {
			return e
		}
		if e := lock.Lock(); e != nil {
			return e
		}
		defer lock.Close()
		c, e := load()
		if os.IsNotExist(e) {
			c = DefaultConfig()
		} else if e != nil {
			return e
		}
		f := Folder{Source: args[0], Target: "~"}
		if len(args) > 1 {
			f.Target = args[1]
		}
		s, e := expand(f.Source)
		if e != nil {
			return e
		}
		t, e := expand(f.Target)
		if e != nil {
			return e
		}
		for _, p := range []string{s, t} {
			info, e := os.Stat(p)
			if e != nil {
				return e
			}
			if !info.IsDir() {
				return fmt.Errorf("not a directory: %s", p)
			}
		}
		for _, old := range c.Folders {
			p, _ := expand(old.Source)
			q, _ := expand(old.Target)
			if samePath(p, s) && samePath(q, t) {
				return nil
			}
		}
		c.Folders = append(c.Folders, f)
		return SaveConfig(configFile, c)
	}}
	repo.AddCommand(add)
	repo.AddCommand(&cobra.Command{Use: "remove <source>", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		lock := flock.New(configFile + ".lock")
		if e := lock.Lock(); e != nil {
			return e
		}
		defer lock.Close()
		c, e := load()
		if e != nil {
			return e
		}
		p, e := expand(args[0])
		if e != nil {
			return e
		}
		var keep []Folder
		found := false
		for _, f := range c.Folders {
			s, _ := expand(f.Source)
			if samePath(s, p) {
				found = true
			} else {
				keep = append(keep, f)
			}
		}
		if !found {
			return fmt.Errorf("source is not configured: %s", p)
		}
		c.Folders = keep
		return SaveConfig(configFile, c)
	}})
	syncCmd := func() *cobra.Command {
		var selected, key string
		var reset bool
		c := &cobra.Command{Use: "sync", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			c, e := load()
			if e != nil {
				return e
			}
			folders, e := c.Selected(selected)
			if e != nil {
				return e
			}
			if key != "" {
				key, e = expand(key)
				if e != nil {
					return e
				}
				if _, e = os.Stat(key); e != nil {
					return e
				}
			}
			results := Sync(cmd.Context(), folders, reset, key)
			for _, r := range results {
				fmt.Fprintf(out, "%s: %s\n%s\n", r.Repo, r.Kind, r.Detail)
			}
			if e = recordResults(state, results); e != nil {
				return e
			}
			if code := resultCode(results); code != 0 {
				return exitCode{code}
			}
			return nil
		}}
		c.Flags().StringVar(&selected, "repo", "", "Limit to a configured source")
		c.Flags().StringVar(&key, "ssh-key", "", "SSH private key")
		c.Flags().BoolVar(&reset, "reset", false, "Replace local state from origin after saving recovery data")
		return c
	}
	root.AddCommand(syncCmd())
	repo.AddCommand(syncCmd())
	for _, name := range []string{"list", "link", "doctor", "create"} {
		name := name
		var selected string
		var dry, force, verbose bool
		cmd := &cobra.Command{Use: name, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
			c, e := load()
			if e != nil {
				return e
			}
			folders, e := c.Selected(selected)
			if e != nil {
				return e
			}
			switch name {
			case "list":
				return List(folders, out)
			case "doctor":
				return Doctor(folders, out)
			case "create":
				if selected == "" {
					return fmt.Errorf("create requires --repo")
				}
				return Create(folders[0], args[0], in, out)
			default:
				e = Link(folders, LinkOptions{dry, force, verbose, in, out})
				detail := ""
				if e != nil {
					detail = e.Error()
				}
				if dry {
					return e
				}
				if se := reportProblem(state, "link", detail); se != nil && e == nil {
					return se
				}
				return e
			}
		}}
		cmd.Flags().StringVar(&selected, "repo", "", "Limit to a configured source")
		if name == "create" {
			cmd.Use = "create <path>"
			cmd.Args = cobra.ExactArgs(1)
		}
		if name == "link" {
			cmd.Flags().BoolVar(&dry, "dry-run", false, "Print changes without applying them")
			cmd.Flags().BoolVarP(&force, "force", "f", false, "Ask before replacing existing paths")
			cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show existing correct links")
		}
		root.AddCommand(cmd)
	}
	var ack bool
	status := &cobra.Command{Use: "status", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error { return printStatus(state, ack, out) }}
	status.Flags().BoolVar(&ack, "ack", false, "Dismiss recovery notices")
	root.AddCommand(status)
	agent := &cobra.Command{Use: "agent", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() }}
	root.AddCommand(agent)
	for _, action := range []string{"uninstall", "run", "status"} {
		action := action
		agent.AddCommand(&cobra.Command{Use: action, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			if action == "uninstall" {
				if e := writeAtomic(filepath.Join(state, "agent-disabled"), []byte("disabled"), 0600); e != nil {
					return e
				}
				return uninstallAgent(cmd.Context(), out)
			}
			if action == "status" {
				e := platformAgentStatus(cmd.Context(), out)
				se := printStatus(state, false, out)
				if e != nil {
					return e
				}
				return se
			}
			c, e := load()
			if e != nil {
				if action == "run" {
					return agentConfigError(cmd.Context(), state, e, out)
				}
				return e
			}
			if len(c.Folders) > 0 {
				if _, e = c.Selected(""); e != nil {
					if action == "run" {
						return agentConfigError(cmd.Context(), state, e, out)
					}
					return e
				}
			}

			return runAgent(cmd.Context(), c, state, out, desktopNotify, version)
		}})
	}
	packageCmd := &cobra.Command{Use: "_package", Hidden: true}
	for _, action := range []string{"setup", "login", "remove"} {
		action := action
		packageCmd.AddCommand(&cobra.Command{Use: action, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			if action == "remove" {
				return removePackageAgent(cmd.Context(), state, out)
			}
			err := setupPackageAgent(cmd.Context(), configFile, state, out, action == "setup")
			if err != nil {
				_ = reportProblem(state, "setup", err.Error())
			} else {
				_ = reportProblem(state, "setup", "")
			}
			return err
		}})
	}
	root.AddCommand(packageCmd)
	self := &cobra.Command{Use: "self"}
	var check bool
	update := &cobra.Command{Use: "update", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return SelfUpdate(cmd.Context(), version, check, out) }}
	update.Flags().BoolVar(&check, "check", false, "Check without installing")
	self.AddCommand(update)
	root.AddCommand(self)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	root.SetContext(ctx)
	if e := root.Execute(); e != nil {
		var code exitCode
		if errors.As(e, &code) {
			return code.code
		}
		fmt.Fprintln(errOut, "Error:", e)
		return 1
	}
	return 0
}
