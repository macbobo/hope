package cli

import (
	"fmt"
	"github.com/dongbo/gope/app"
	"github.com/dongbo/gope/config"
	"github.com/dongbo/gope/test"
	"github.com/dongbo/gope/utils"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	_ "net/http/pprof"
	"os"
)

const MAIN_VERSION = "v0.0.1-alpha"

var config_cli config.Config_t

var rootCmd = &cobra.Command{
	Use:   "gope",
	Short: "gope是使用golang实现的网络代理与交换服务程序",
	Long: "gope是使用golang实现的网络代理与交换服务程序\n" +
		"1> 默认CLI运行模式，配合全局参数配置使用\n" +
		"2> config子命令：配置文件默认为程序运行目录./gope.conf(), 通过--file自定义配置文件路径\n" +
		"3> unittest子命令：运行test目录单元测试用例，通过--unit指定具体用例\n",
	Version: MAIN_VERSION,
	//ValidArgs: []string{"--help", "--file", "--deamon", "--"},
	Run: func(cmd *cobra.Command, args []string) {
		if cap(args) == 0 {
			cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {

	rootCmd.AddCommand(test.TestCmd)
	rootCmd.AddCommand(config.ConfigCmd)

	rootCmd.PersistentFlags().StringVarP(&config_cli.App, "app", "a", "", "proxy application")
	rootCmd.PersistentFlags().StringVarP(&config_cli.Protocol_str, "protocol", "p", "tcp", "proxy protocol")
	rootCmd.PersistentFlags().StringVarP(&config_cli.Bindip_str, "proxyip", "i", config.LOCALIP_STR, "proxy ip")
	rootCmd.PersistentFlags().Uint16VarP(&config_cli.Bindport, "proxyport", "t", 54321, "proxy port")

	rootCmd.PersistentFlags().StringVarP(&config_cli.Upstreamip_str, "upstreamip", "I", config.LOCALIP_STR, "dst ip")
	rootCmd.PersistentFlags().Uint16VarP(&config_cli.Upstreamport, "upstreamport", "T", 12345, "dst port")
	rootCmd.PersistentFlags().StringVar(&config_cli.Protocolex_str, "upstreamproto", "", "dst protocol")
	rootCmd.PersistentFlags().StringVar(&config_cli.Appex, "appx", "", "dst application")

}

func Run() {
	cobra.CheckErr(rootCmd.Execute())

	if len(config.Configfile.Configs) > 0 {
		utils.Logger.SugarLogger.Infof("conffile total: %d", len(config.Configfile.Configs))
		return
	}

	utils.Logger = utils.Newloger(zap.DebugLevel, "./gope.log", true)
	defer func() {
	}()

	//检查服务配置
	config.Verbose(&config_cli)

	utils.Logger.SugarLogger.Infof("subconfig total: %d", len(config_cli.Subext))
	for _, sub := range config_cli.Subext {
		config.Verbose(&sub)
	}

	r, err := config.Checkconfig(&config_cli, true)
	if r < 0 {
		fmt.Println(err)
	}

	//gnet 启动代理与交换服务
	s := new(app.Tcp_udp_s)
	s.Config = config_cli

	s.Init()
	s.Startup()
	s.Wait()
}
