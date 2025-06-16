package proxy

import (
	"fmt"
	"github.com/rs/xid"
	"github.com/spf13/cobra"
	"github.com/unibackend/uniproxy/internal"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/config/reader"
	"log"
)

var Run = &cobra.Command{
	Use:   "proxy",
	Short: "Run proxy",
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		log.Println("Proxy", common.UniProxyVersion, "launching...")
		configPath, err := cmd.Flags().GetString("config")
		if err != nil {
			log.Panicf("Cannot parse config path: %s", err)
		}

		var cfg *config.Config

		// Read config from file
		if cfg, err = reader.FromFile(configPath); err != nil {
			return
		}

		// Read environment variables into config
		reader.PrepareFromEnv(cfg)

		// Define ID of proxy
		if cfg.ProxyId == "" {
			cfg.ProxyId = xid.New().String()
			if proxyId, err := cmd.Flags().GetString("id"); err == nil {
				cfg.ProxyId = proxyId
			}
			cfg.ProxyId = fmt.Sprintf("proxy-%s", cfg.ProxyId)
		}

		mode := common.DiModeProxy
		if proxyMode, err := cmd.Flags().GetString("mode"); err == nil && proxyMode != "" {
			mode = proxyMode
		}

		// Create and init uber container
		container, err := internal.NewContainer(cfg, mode)
		if err != nil {
			log.Panicf("Cannot build container: %v", err)
		}

		// Background worker
		go func() {
			if err := container.RunWorker(mode); err != nil {
				log.Panicf("Cannot run worker: %v", err)
			}
		}()

		return container.RunProxy()
	},
}
