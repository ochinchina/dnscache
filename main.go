package main

import (
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"strings"
)

type DNSCacheConfig struct {
	Caches []struct {
		ListenAddrs []string `yaml:"listenAddrs,omitempty"`
		DnsServers  []string `yaml:"dnsServers,omitempty"`
	} `yaml:"caches,omitempty"`
}

func initLogger(logFile string, logLevel string, logFormat string, logSize int, backups int) {
	var logEncoder zapcore.Encoder
	if strings.ToLower(logFormat) == "json" {
		logEncoder = zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	} else {
		logEncoder = zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	}
	level := zapcore.DebugLevel
	level.Set(logLevel)
	highPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= level
	})

	out := &lumberjack.Logger{Filename: logFile,
		LocalTime:  true,
		MaxSize:    logSize,
		MaxBackups: backups}

	core := zapcore.NewCore(logEncoder, zapcore.AddSync(out), highPriority)

	logger := zap.New(core)
	zap.ReplaceGlobals(logger)
}

func loadConfigFromReader(reader io.Reader) (*DNSCacheConfig, error) {
	r := &DNSCacheConfig{}

	decoder := yaml.NewDecoder(reader)
	err := decoder.Decode(r)

	if err != nil {
		return nil, err
	}

	return r, nil
}

func loadConfigFromFile(fileName string) (*DNSCacheConfig, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	defer f.Close()
	return loadConfigFromReader(f)

}
func startDNSCacheServer(c *cli.Context) error {
	fileName := c.String("config")
	initLogger(c.String("log-file"), c.String("log-level"), c.String("log-format"), c.Int("log-size"), c.Int("log-backups"))
	config, err := loadConfigFromFile(fileName)
	if err != nil {
		zap.L().Error("Fail to load the configuration from file", zap.String("file", fileName))
		return err
	}
	for _, cache := range config.Caches {
		err := NewCacheServer(cache.ListenAddrs, cache.DnsServers).start()
		if err != nil {
			return err
		}
	}
	return nil

}
func main() {
	app := &cli.App{
		Name:  "dnscache",
		Usage: "DNS server record cache",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "config",
				Aliases:  []string{"c"},
				Required: true,
				Usage:    "Load configuration from `FILE`",
			},
			&cli.StringFlag{
				Name:  "log-format",
				Usage: "log file format: json or console",
			},
			&cli.StringFlag{
				Name:  "log-file",
				Usage: "log file name",
			},
			&cli.StringFlag{
				Name:  "log-level",
				Usage: "one of following level: Debug, Info, Warn, Error, Fatal, Panic",
			},
			&cli.IntFlag{
				Name:  "log-size",
				Usage: "size of log file in Megabytes",
				Value: 50,
			},
			&cli.IntFlag{
				Name:  "log-backups",
				Usage: "number of log rotate files",
				Value: 10,
			},
		},
		Action: startDNSCacheServer,
	}
	err := app.Run(os.Args)
	if err != nil {
		zap.L().Error("Fail to start application", zap.String("error", err.Error()))
	}
}
