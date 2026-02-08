package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"ironclaw/internal/banner"
	"ironclaw/internal/brain"
	"ironclaw/internal/cli"
	"ironclaw/internal/config"
	"ironclaw/internal/gateway"
	"ironclaw/internal/llm"
	"ironclaw/internal/memory"
	"ironclaw/internal/prefs"
	"ironclaw/internal/scheduler"
	"ironclaw/internal/secrets"
	"ironclaw/internal/security"
)

// buildMeta holds version and build metadata (injectable via ldflags).
type buildMeta struct {
	Version string
	GoOS    string
	GoArch  string
}

func newBuildMeta(version, goos, goarch string) buildMeta {
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	return buildMeta{Version: version, GoOS: goos, GoArch: goarch}
}

func (m buildMeta) String() string {
	return fmt.Sprintf("ironclaw %s %s/%s", m.Version, m.GoOS, m.GoArch)
}

func newRootCommand(bm buildMeta) *cobra.Command {
	root := &cobra.Command{
		Use:   "ironclaw",
		Short: "Agent framework",
		Long:  "Ironclaw is a local-first agent framework.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion, _ := cmd.Flags().GetBool("version"); showVersion {
				fmt.Fprintln(cmd.OutOrStdout(), bm.String())
				return nil
			}
			return runDaemon(cmd, args, daemonShutdownCh)
		},
	}
	root.Flags().BoolP("version", "V", false, "print version and build metadata")

	checkCmd := &cobra.Command{
		Use:   "check",
		Short: "Check config, gateway, and paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			fix, _ := cmd.Flags().GetBool("fix")
			checkArgs := []string{"ironclaw", "check"}
			if fix {
				checkArgs = append(checkArgs, "--fix")
			}
			code := cli.RunCheck(checkArgs, cmd.OutOrStdout(), cmd.ErrOrStderr())
			if code != 0 {
				return exitCodeErr(code)
			}
			return nil
		},
	}
	checkCmd.Flags().Bool("fix", false, "write default config if missing")
	root.AddCommand(checkCmd)

	polishCmd := &cobra.Command{
		Use:     "polish",
		Short:   "Nail polish mode",
		Aliases: []string{"💅🏼"},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), cli.NailPolish)
			return nil
		},
	}
	root.AddCommand(polishCmd)

	prefsCmd := &cobra.Command{Use: "prefs", Short: "Get or set user preferences (theme, defaultModel)"}
	getCmd := &cobra.Command{Use: "get", Short: "Get a preference value", RunE: runPrefsGet}
	getCmd.Args = cobra.ExactArgs(1)
	setCmd := &cobra.Command{Use: "set", Short: "Set a preference value", RunE: runPrefsSet}
	setCmd.Args = cobra.ExactArgs(2)
	prefsCmd.AddCommand(getCmd, setCmd)
	root.AddCommand(prefsCmd)

	secretsCmd := &cobra.Command{Use: "secrets", Short: "Store or retrieve API keys and secrets (encrypted, not in config)"}
	secretsSetCmd := &cobra.Command{Use: "set", Short: "Store a secret (e.g. API key) by name", RunE: runSecretsSet}
	secretsSetCmd.Args = cobra.ExactArgs(2)
	secretsGetCmd := &cobra.Command{Use: "get", Short: "Retrieve a secret by name", RunE: runSecretsGet}
	secretsGetCmd.Args = cobra.ExactArgs(1)
	secretsDeleteCmd := &cobra.Command{Use: "delete", Short: "Remove a secret by name", RunE: runSecretsDelete}
	secretsDeleteCmd.Args = cobra.ExactArgs(1)
	secretsCmd.AddCommand(secretsSetCmd, secretsGetCmd, secretsDeleteCmd)
	root.AddCommand(secretsCmd)

	return root
}

func runPrefsGet(cmd *cobra.Command, args []string) error {
	path, err := prefs.ConfigPath()
	if err != nil {
		return err
	}
	m := prefs.NewManager(path)
	if err := m.Load(); err != nil {
		return err
	}
	c := m.Config()
	key := args[0]
	switch key {
	case "theme":
		fmt.Fprintln(cmd.OutOrStdout(), c.Theme)
	case "defaultModel":
		fmt.Fprintln(cmd.OutOrStdout(), c.DefaultModel)
	default:
		return fmt.Errorf("unknown key %q", key)
	}
	return nil
}

func runPrefsSet(cmd *cobra.Command, args []string) error {
	path, err := prefs.ConfigPath()
	if err != nil {
		return err
	}
	m := prefs.NewManager(path)
	if err := m.Load(); err != nil {
		return err
	}
	if err := m.SetPreference(args[0], args[1]); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "ok")
	return nil
}

func runSecretsSet(cmd *cobra.Command, args []string) error {
	m, err := secrets.DefaultManager()
	if err != nil {
		return err
	}
	key, value := args[0], args[1]
	if err := m.Set(key, value); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "ok")
	return nil
}

func runSecretsGet(cmd *cobra.Command, args []string) error {
	m, err := secrets.DefaultManager()
	if err != nil {
		return err
	}
	value, err := m.Get(args[0])
	if err != nil {
		if err == secrets.ErrNotFound {
			return fmt.Errorf("secret %q not found", args[0])
		}
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), value)
	return nil
}

func runSecretsDelete(cmd *cobra.Command, args []string) error {
	m, err := secrets.DefaultManager()
	if err != nil {
		return err
	}
	return m.Delete(args[0])
}

// runDaemon runs the daemon loop. If shutdownCh is non-nil, it returns when shutdownCh is closed (for tests).
// Otherwise it blocks on OS signals.
func runDaemon(cmd *cobra.Command, args []string, shutdownCh <-chan struct{}) error {
	euidGetter := security.EffectiveUIDGetter()
	if daemonEUIDGetter != nil {
		euidGetter = daemonEUIDGetter
	}
	if err := security.RequireNonRoot(euidGetter); err != nil {
		return err
	}
	version := getVersion()
	banner.Startup(version, nil)

	// Load user preferences from config dir (UserConfigDir/ironclaw/config.json)
	if path, err := prefs.ConfigPath(); err == nil {
		_ = prefs.NewManager(path).Load()
	}

	cfgPath := os.Getenv("IRONCLAW_CONFIG")
	if cfgPath == "" {
		cfgPath = "ironclaw.json"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Println("  (no config file, using defaults)")
	} else {
		fmt.Printf("  gateway :%d  auth=%s\n", cfg.Gateway.Port, cfg.Gateway.Auth.Mode)
	}

	var gatewayShutdown chan struct{}
	var sched *scheduler.Scheduler
	if cfg != nil {
		var chatBrain *brain.Brain
		if sm, err := secrets.DefaultManager(); err == nil {
			getSecret := sm.Get
			provider, err := llm.NewProvider(&cfg.Agents, getSecret, &cfg.Retry)
			if err == nil {
				var opts []brain.Option
				if cfg.Agents.Paths.Memory != "" {
					memStore := memory.NewFileMemoryStore(cfg.Agents.Paths.Memory)
					opts = append(opts, brain.WithMemory(memStore))
				}
				if len(cfg.Agents.Fallbacks) > 0 {
					fallbacks := llm.NewFallbackProviders(cfg.Agents.Fallbacks, getSecret, &cfg.Retry)
					if len(fallbacks) > 0 {
						opts = append(opts, brain.WithFallbacks(fallbacks...))
					}
				}
				chatBrain = brain.NewBrain(provider, opts...)
			}
		}

		// Initialize the scheduler with the brain as the event handler.
		if chatBrain != nil {
			engine := scheduler.NewRobfigCronEngine()
			handler := makeSchedulerHandler(chatBrain, schedulerPrintFn)
			sched = scheduler.NewScheduler(engine, handler)
			sched.Start()
			fmt.Println("  scheduler started")
		}

		srv, srvErr := gateway.NewServer(&cfg.Gateway, chatBrain)
		if srvErr != nil {
			fmt.Fprintf(gatewayBindErrWriter, "  gateway start: %v\n", srvErr)
		} else {
			gatewayServerForTest = srv
			gatewayShutdown = make(chan struct{})
			go func() {
				_ = srv.Run(gatewayShutdown)
			}()
			// Wait until the server has bound so "ready." means clients can connect.
			var bound string
			for i := 0; i < daemonBindWaitIterations; i++ {
				if a := srv.Addr(); a != "" {
					bound = a
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			if bound != "" {
				fmt.Printf("  listen %s\n  ready.\n", bound)
			} else {
				if err := srv.ListenErr(); err != nil {
					fmt.Fprintf(gatewayBindErrWriter, "  gateway failed to bind: %v\n", err)
				} else {
					fmt.Fprintln(gatewayBindErrWriter, "  gateway failed to bind (check port or permissions)")
				}
			}
		}
	}
	if gatewayShutdown == nil {
		fmt.Println("  ready.")
	}

	if shutdownCh != nil {
		<-shutdownCh
		if sched != nil {
			sched.Stop()
		}
		if gatewayShutdown != nil {
			close(gatewayShutdown)
		}
		return nil
	}
	daemonWaitForShutdown()
	if sched != nil {
		sched.Stop()
	}
	if gatewayShutdown != nil {
		close(gatewayShutdown)
	}
	return nil
}

// schedulerPrintFn controls where scheduler handler output goes. Tests override this.
var schedulerPrintFn = func(format string, args ...any) {
	fmt.Printf(format, args...)
}

// makeSchedulerHandler creates an EventHandler that injects the cron job's prompt
// into the brain as a system event. printFn is used for output (testable).
func makeSchedulerHandler(b *brain.Brain, printFn func(string, ...any)) scheduler.EventHandler {
	return func(ctx context.Context, job scheduler.Job) error {
		systemPrompt := fmt.Sprintf("[System Event: Scheduled Job %q]\n%s", job.Name, job.Prompt)
		resp, err := b.Generate(ctx, systemPrompt)
		if err != nil {
			printFn("  scheduler: job %q error: %v\n", job.ID, err)
			return err
		}
		printFn("  scheduler: job %q response: %s\n", job.ID, resp)
		return nil
	}
}

func getVersion() string {
	if version != "" {
		return version
	}
	b, err := os.ReadFile("VERSION")
	if err != nil {
		return "dev"
	}
	return strings.TrimSpace(string(b))
}

// version is set at build time via ldflags for build metadata, e.g.:
//   go build -ldflags "-X main.version=1.0.8" -o ironclaw ./cmd/ironclaw
var version string

// daemonShutdownCh is set by tests to unblock runDaemon without signals. Production leaves it nil.
var daemonShutdownCh <-chan struct{}

// daemonEUIDGetter is set by tests to avoid RequireNonRoot failing when test runs as root. Production leaves it nil.
var daemonEUIDGetter func() int

// daemonWaitForShutdown is set by init in main_signal*.go so tests can inject a no-op to cover the nil-shutdownCh path.
var daemonWaitForShutdown func()

// gatewayServerForTest is set when the gateway server starts so tests can read Addr().
var gatewayServerForTest *gateway.Server

// daemonBindWaitIterations is the max loop count waiting for gateway to bind. Tests may set to 0 to skip wait and cover the "failed to bind (check port or permissions)" branch.
var daemonBindWaitIterations = 50

// gatewayBindErrWriter is where bind errors are written. Tests set this to capture output; production uses os.Stderr.
var gatewayBindErrWriter interface{ Write([]byte) (int, error) } = os.Stderr

// exitCodeErr carries an exit code for the process. When returned from a command, runApp exits with that code.
type exitCodeErr int

func (e exitCodeErr) Error() string { return fmt.Sprintf("exit %d", int(e)) }
func (e exitCodeErr) ExitCode() int { return int(e) }

// runApp runs the root command with the given args and returns the exit code (0, 1, or 2).
func runApp(args []string) int {
	bm := newBuildMeta(version, "", "")
	if bm.Version == "" {
		bm.Version = getVersion()
	}
	root := newRootCommand(bm)
	root.SetArgs(args[1:])
	if err := root.Execute(); err != nil {
		if err == security.ErrRunningAsRoot {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		if ec, ok := err.(interface{ ExitCode() int }); ok {
			return ec.ExitCode()
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

