package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/yonnic/goshop/common"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Info",
	Long:  `Info`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%v v%v, build: %v succesfully installed\n", common.APPLICATION, common.VERSION, common.COMPILED)
		if common.Config.Https.Enabled {
			fmt.Printf("Listen on https://%v:%d/\n", common.Config.Https.Host, common.Config.Https.Port)
		}
	},
}

func init() {
	RootCmd.AddCommand(infoCmd)
}