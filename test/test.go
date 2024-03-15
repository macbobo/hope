package test

import (
	"github.com/spf13/cobra"
	"os"
)

var TestCmd = &cobra.Command{
	Use:   "unittest",
	Short: "单元测试",
	//ValidArgs: []string{"--help", "--file", "--deamon", "--"},
	Run: func(cmd *cobra.Command, args []string) {
		if cap(args) == 0 {
			cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {

}
