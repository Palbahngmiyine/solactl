package cmd

import (
	"context"
	"io"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/solapi/solactl/internal/version"
	"github.com/solapi/solactl/pkg/client"
	"github.com/solapi/solactl/pkg/config"
	"github.com/solapi/solactl/pkg/logger"
	"github.com/solapi/solactl/pkg/output"
)

var (
	flagAPIKey    string
	flagAPISecret string
	flagProfile   string
	flagJSON      bool
	flagDebug     bool
	flagTimeout   time.Duration
)

var (
	cmdCtx       context.Context
	cmdCtxCancel context.CancelFunc
)

// outWriter is the destination for command output. Tests override this.
var outWriter io.Writer

// errWriter is the destination for informational/diagnostic output (stderr).
// Tests override this to capture stderr messages.
var errWriter io.Writer

// clientOverride is set by tests to bypass loadConfig and use a test client.
var clientOverride *client.Client

var rootCmd = &cobra.Command{
	Use:   "solactl",
	Short: "SOLAPI CLI — 메시지 발송 및 관리 도구",
	Long:  "SOLAPI API를 CLI로 제어합니다. 메시지 발송, 발신번호 관리, 잔액 조회 등을 지원합니다.",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logger.Init(flagDebug)
		sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		if flagTimeout > 0 {
			cmdCtx, cmdCtxCancel = context.WithTimeout(sigCtx, flagTimeout)
			origCancel := cmdCtxCancel
			cmdCtxCancel = func() { origCancel(); stop() }
		} else {
			cmdCtx, cmdCtxCancel = sigCtx, stop
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagAPIKey, "api-key", "", "API Key")
	rootCmd.PersistentFlags().StringVar(&flagAPISecret, "api-secret", "", "API Secret")
	rootCmd.PersistentFlags().StringVar(&flagProfile, "profile", "", "사용할 프로필 이름")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "JSON 출력 모드")
	rootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "디버그 로그 출력")
	rootCmd.PersistentFlags().DurationVar(&flagTimeout, "timeout", 30*time.Second, "요청 타임아웃 (예: 30s, 1m)")
}

// Execute runs the root command and ensures context cleanup.
func Execute() error {
	defer func() {
		if cmdCtxCancel != nil {
			cmdCtxCancel()
		}
	}()
	return rootCmd.Execute()
}

// loadConfig loads and validates the CLI configuration.
func loadConfig() (*config.Config, error) {
	overrides := &config.Config{}
	if flagAPIKey != "" {
		overrides.APIKey = flagAPIKey
	}
	if flagAPISecret != "" {
		overrides.APISecret = flagAPISecret
	}

	cfg, err := config.Load(&config.LoadOptions{
		Overrides:   overrides,
		ProfileName: flagProfile,
	})
	if err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// newClient creates a SOLAPI REST client from the current configuration.
func newClient() (*client.Client, error) {
	if clientOverride != nil {
		return clientOverride, nil
	}
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	logger.Debug("SOLAPI 클라이언트 생성: %s", client.BaseURL)
	c := client.New(cfg.APIKey, cfg.APISecret)
	c.UserAgent = "solactl/" + version.Version + " (" + runtime.GOOS + "/" + runtime.GOARCH + ")"
	return c, nil
}

// out returns the current output writer, falling back to os.Stdout.
func out() io.Writer {
	if outWriter != nil {
		return outWriter
	}
	return os.Stdout
}

// errOut returns the current error/diagnostic writer, falling back to os.Stderr.
func errOut() io.Writer {
	if errWriter != nil {
		return errWriter
	}
	return os.Stderr
}

// printer returns a configured output printer.
func printer() *output.Printer {
	return &output.Printer{Writer: out(), JSONMode: flagJSON}
}

// ctx returns the command-scoped context.
func ctx() context.Context {
	if cmdCtx != nil {
		return cmdCtx
	}
	return context.Background()
}
