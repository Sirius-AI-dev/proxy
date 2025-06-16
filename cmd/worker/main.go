package worker

//
//import (
//	"fmt"
//	"github.com/rs/xid"
//	"github.com/spf13/cobra"
//	"github.com/unibackend/uniproxy/internal"
//	"github.com/unibackend/uniproxy/internal/common"
//	configReader "github.com/unibackend/uniproxy/internal/config"
//	"log"
//)
//
//var Run = &cobra.Command{
//	Use:   "worker",
//	Short: "Run worker",
//	RunE: func(cmd *cobra.Command, args []string) error {
//		log.Println("Worker", common.UniProxyVersion, "launching...")
//		configPath, err := cmd.Flags().GetString("config")
//		if err != nil {
//			log.Panicf("Cannot parse config path: %s", err)
//		}
//
//		cfg := configReader.ParseConfig(configPath)
//		cfg.ProxyId = fmt.Sprintf("worker-%s", xid.New())
//
//		container, err := internal.NewContainer(cfg, common.DiModeWorker)
//		if err != nil {
//			log.Panicf("Cannot build container: %v", err)
//		}
//
//		return container.RunWorker(common.DiModeWorker)
//	},
//}
