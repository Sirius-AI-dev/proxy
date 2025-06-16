package main

import (
	"github.com/spf13/cobra"
	"github.com/unibackend/uniproxy/cmd/proxy"
)

var rootCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Proxy",
}

func main() {
	rootCmd.AddCommand(proxy.Run)

	rootCmd.PersistentFlags().StringP("config", "c", "", "--config=config/config.yml")
	rootCmd.PersistentFlags().StringP("id", "i", "", "--id=1")
	rootCmd.PersistentFlags().StringP("mode", "m", "", "--mode=worker")

	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}
