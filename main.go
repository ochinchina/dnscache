package main

import (
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"io"
	"os"
)

type DNSCacheConfig struct {
	ListenAddr string `yaml:"listenAddr,omitempty"`
	DnsServers []string `yaml:"dnsServers,omitempty"`
}

func initLogger(logFile string, logLevel string) {
	logger, _ := zap.NewDevelopment()
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
	initLogger( c.String("log-file" ), c.String("log-level"))
	config, err := loadConfigFromFile(fileName)
	if err != nil {
		zap.L().Error("Fail to load the configuration from file", zap.String("file", fileName))
		return err
	}
	return NewCacheServer(config.ListenAddr, config.DnsServers).start()

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
				Name:  "log-file",
				Usage: "log file name",
			},
			&cli.StringFlag{
				Name:  "log-level",
				Usage: "one of following level: Trace, Debug, Info, Warn, Error, Fatal, Panic",
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
