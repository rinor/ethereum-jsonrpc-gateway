package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type Config struct {
	Upstreams                 []string `json:"upstreams"`
	SendRawTransactionUrl     string   `json:"sendRawTransactionUrl"`
	OldTrieUrl                string   `json:"oldTrieUrl"`
	Strategy                  string   `json:"strategy"`
	MethodLimitationEnabled   bool     `json:"methodLimitationEnabled"`
	AllowedMethods            []string `json:"allowedMethods"`
	ContractLimitationEnabled bool     `json:"contractLimitationEnabled"`
	AllowedContracts          []string `json:"allowedContracts"`
}

type RunningConfig struct {
	ctx                       context.Context
	stop                      context.CancelFunc
	Upstreams                 []Upstream
	Strategy                  IStrategy
	MethodLimitationEnabled   bool
	ContractLimitationEnabled bool
	allowedMethods            map[string]bool
	allowedCallContracts      map[string]bool
}

var currentConfigString string = ""
var currentRunningConfig *RunningConfig

func LoadConfig(ctx context.Context, quit chan bool) {

	ticker := time.NewTicker(3 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				config := &Config{}

				logrus.Debugf("load config from file")
				bts, err := os.ReadFile("./config.json")

				if err != nil {
					if currentConfigString == "" {
						logrus.Fatal(err)
					} else {
						logrus.Warn("hot read config err, use old config")
						continue
					}
				}

				if string(bts) != currentConfigString {
					_ = json.Unmarshal(bts, config)

					currentRunningConfig, err = BuildRunningConfigFromConfig(ctx, config)

					if err != nil {
						if currentConfigString == "" {
							logrus.Fatal(err)
						} else {
							logrus.Warn("hot build config err, use old config")
							continue
						}
					}

					currentConfigString = string(bts)
				}
			case <-quit:
				logrus.Info("quit loop config")
				ticker.Stop()
				return
			}
		}
	}()
}

func BuildRunningConfigFromConfig(parentContext context.Context, cfg *Config) (*RunningConfig, error) {
	ctx, stop := context.WithCancel(parentContext)

	rcfg := &RunningConfig{
		ctx:  ctx,
		stop: stop,
	}

	currentRunningConfig = rcfg

	for _, url := range cfg.Upstreams {

		var primaryUrl string
		var oldTrieUrl string
		var sendRawTransactionUrl string

		primaryUrl = url

		if cfg.SendRawTransactionUrl != "" {
			sendRawTransactionUrl = cfg.SendRawTransactionUrl
		} else {
			sendRawTransactionUrl = url
		}

		if cfg.OldTrieUrl != "" {
			oldTrieUrl = cfg.OldTrieUrl
		} else {
			oldTrieUrl = url
		}

		rcfg.Upstreams = append(rcfg.Upstreams, newUpstream(ctx, primaryUrl, oldTrieUrl, sendRawTransactionUrl))
	}

	if len(rcfg.Upstreams) == 0 {
		return nil, fmt.Errorf("need upstreams")
	}

	switch cfg.Strategy {
	case "NAIVE":
		if len(rcfg.Upstreams) > 1 {
			panic(fmt.Errorf("naive proxy strategy require exact 1 upstream"))
		}
		rcfg.Strategy = newNaiveProxy()
	case "RACE":
		if len(rcfg.Upstreams) < 2 {
			panic(fmt.Errorf("race proxy strategy require more than 1 upstream"))
		}
		rcfg.Strategy = newRaceProxy()
	case "FALLBACK":
		if len(rcfg.Upstreams) < 2 {
			panic(fmt.Errorf("fallback proxy strategy require more than 1 upstream"))
		}
		rcfg.Strategy = newFallbackProxy()
	default:
		return nil, fmt.Errorf("blank of unsupported strategy: %s", cfg.Strategy)
	}

	rcfg.MethodLimitationEnabled = cfg.MethodLimitationEnabled

	rcfg.allowedMethods = make(map[string]bool)
	for i := 0; i < len(cfg.AllowedMethods); i++ {
		rcfg.allowedMethods[cfg.AllowedMethods[i]] = true
	}

	rcfg.ContractLimitationEnabled = cfg.ContractLimitationEnabled

	rcfg.allowedCallContracts = make(map[string]bool)
	for i := 0; i < len(cfg.AllowedContracts); i++ {
		rcfg.allowedCallContracts[strings.ToLower(cfg.AllowedContracts[i])] = true
	}

	return rcfg, nil
}
