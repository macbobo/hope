package config

import (
	"errors"
	"fmt"
	"github.com/macbobo/gope/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"net"
	"os"
	"time"
)

var ConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "通过配置文件启动多个网络代理与交换服务",
	//ValidArgs: []string{"--help", "--file", "--deamon", "--"},
	Run: func(cmd *cobra.Command, args []string) {

		if _, err := os.Stat(Configfile.confpath); os.IsNotExist(err) {
			fmt.Println(err)
			os.Exit(1)
		}

		viper.SetConfigFile(Configfile.confpath)
		viper.SetConfigType("yaml")
		err := viper.ReadInConfig()
		if err != nil {
			fmt.Println("config", Configfile.confpath, err)
			os.Exit(1)
		}

		/**
		配置文件样例sample如下
		proxy:
			- id: 1
			app:
			inputip:
			inputport: 			//支持多端口定义",-"
			inputproto: tcp
			outputip:
			outputport: 		//支持多端口定义",-"
			outputproto: tcp

		*/
		v := viper.Get("proxy")
		fmt.Println(v)

		for _, item := range v.([]interface{}) {
			utils.Logger.SugarLogger.Debugf("########################################################################")

			var cn Config_t
			var ports1, ports2 []int

			for k, v := range item.(map[interface{}]interface{}) {
				utils.Logger.SugarLogger.Debugf("%s: %v\n", k, v)
				switch v.(type) {
				case int:
					utils.Logger.SugarLogger.Debug("类型: int，值:", v.(int))
				case string:
					utils.Logger.SugarLogger.Debug("类型: string，值:", v.(string))
				case float64:
					utils.Logger.SugarLogger.Debug("类型: float64，值:", v.(float64))
				default:
					utils.Logger.SugarLogger.Debug("类型未找到")
				}
			}

			ci := item.(map[interface{}]interface{})

			if t := ci["app"]; t != nil {
				cn.App, _ = t.(string) //_是为了防止类型错误导致的panic
			}

			if t := ci["inputip"]; t != nil {
				cn.Bindip_str, _ = t.(string)
			}

			if t := ci["inputport"]; t != nil {

				//解析分隔符‘，-’
				switch t.(type) {
				case int:
					ports1 = append(ports1, t.(int))
				case string:
					if ports1, err = utils.Portrane(t.(string)); err != nil {
						fmt.Println(t.(string), "is not ports range")
					}
				default:
					utils.Logger.SugarLogger.Warnf("inputport 类型错误 %T\n", t)
				}

			}

			if t := ci["inputproto"]; t != nil {
				cn.Protocol_str, _ = t.(string)
			}

			if t := ci["outputip"]; t != nil {
				cn.Upstreamip_str, _ = t.(string)
			}

			if t := ci["outputport"]; t != nil {

				//解析分隔符‘，-’
				switch t.(type) {
				case int:
					ports2 = append(ports2, t.(int))
				case string:
					if ports2, err = utils.Portrane(t.(string)); err != nil {
						utils.Logger.SugarLogger.Debugf(t.(string), "is not ports range")
					}
				default:
					utils.Logger.SugarLogger.Debugf("outputport 类型错误 %T\n", t)
				}
			}

			if t := ci["outputproto"]; t != nil {
				cn.Protocolex_str, _ = t.(string)
			}

			/**
			端口支持P2P, N*1
			1*N 可以理解为负责均衡(或者流量复制）暂不支持
			N*M 与1*N同理
			*/
			if (len(ports1) > 0) && ((len(ports1) == len(ports2)) || (len(ports2) == 1)) {

				for i := 0; i < len(ports1); i++ {
					cn.Bindport = uint16(ports1[i])
					if i >= len(ports2) {
						cn.Upstreamport = uint16(ports2[0])
					} else {
						cn.Upstreamport = uint16(ports2[i])
					}
					if _, err := Checkconfig(&cn, false); err == nil {
						Configfile.Configs = append(Configfile.Configs, cn)
					} else {
						utils.Logger.SugarLogger.Error(cn, err)
					}
				}

			}

		}

		utils.Logger.SugarLogger.Debug("########################################################################")

	},
}

type config_f struct {
	confpath string
	Configs  []Config_t
}

var Configfile config_f

func init() {
	ConfigCmd.PersistentFlags().StringVarP(&Configfile.confpath, "file", "f", "gope.conf", "配置文件路径")
}

func Verbose(c *Config_t) {

	fmt.Println(*c)
}

func (c Config_t) Verbose() {

	fmt.Println(c)
}

/**

 */
func Checkconfig(c *Config_t, donet bool) (r int, err error) {

	r = 0
	if c == nil {
		r = -1
		err = errors.New("config is nil")
		return
	}

	if len(c.Protocolex_str) == 0 {
		c.Protocolex_str = c.Protocol_str
	}

	if (len(c.App) != 0) && (len(c.Appex) == 0) {
		c.Appex = c.App
	}

	c.Bindip = net.ParseIP(c.Bindip_str)
	if (c.Bindport == 0) || (c.Bindport >= 65535) || (c.Bindip == nil) {
		r = -1
		err = errors.New("config proxy error")
		return
	}
	c.Upstreamip = net.ParseIP(c.Upstreamip_str)
	if (c.Upstreamport == 0) || (c.Upstreamport >= 65535) || (c.Upstreamip == nil) {
		r = -1
		err = errors.New("config upstream error")
		return
	}

	src1 := net.JoinHostPort(c.Bindip_str, fmt.Sprint(c.Bindport))
	src2 := net.JoinHostPort(c.Upstreamip_str, fmt.Sprint(c.Upstreamport))
	src1 = c.Protocol_str + "://" + src1
	src2 = c.Protocolex_str + "://" + src2

	if src1 == src2 {
		r = -1
		err = errors.New("config conflicted error")
		return
	}

	switch c.Protocol_str {
	case "tcp", "tcp4", "tcp6":
		c.Protocol = PROTOCOL_TCP
	case "udp", "udp4", "udp6":
		c.Protocol = PROTOCOL_UDP
	default:
		c.Protocol = -1
	}

	switch c.Protocolex_str {
	case "tcp":
		c.Protocolex = PROTOCOL_TCP
	case "udp":
		c.Protocolex = PROTOCOL_UDP
	default:
		c.Protocolex = -1
	}

	if c.Createtime.IsZero() {
		c.Createtime = time.Now()
	}
	Verbose(c)

	if donet {

		switch c.Protocol {
		case PROTOCOL_TCP:
			{
				addr, _ := net.ResolveTCPAddr(c.Protocol_str, net.JoinHostPort(c.Bindip_str, fmt.Sprint(c.Bindport)))

				ld, err := net.ListenTCP(c.Protocol_str, addr)
				if err != nil {
					r = -2
					return r, err
				}

				defer ld.Close()
			}

		case PROTOCOL_UDP:
			{
				addr, _ := net.ResolveUDPAddr(c.Protocol_str, net.JoinHostPort(c.Bindip_str, fmt.Sprint(c.Bindport)))
				ln, err := net.ListenUDP(c.Protocol_str, addr)
				if err != nil {
					r = -2
					return r, err
				}

				defer ln.Close()
			}

		default:
			{
				r = -1
				err = errors.New("config proxy error, protocol need set to \"tcp or udp\"")
				return
			}
		}

	}

	if donet && ((c.Protocolex == PROTOCOL_TCP) || (c.Protocolex == PROTOCOL_UDP)) {

		cn, err := net.Dial(c.Protocolex_str, net.JoinHostPort(c.Upstreamip_str, fmt.Sprint(c.Upstreamport)))
		if err != nil {
			r = -2
			return r, err
		}

		defer cn.Close()

	} else if donet {
		r = -1
		err = errors.New("config upstream error, protocol need set to \"tcp or udp\"")
	}

	return
}

func (c config_f) Checkconfig() (r int, err error) {

	for i, v := range c.Configs {
		r, err = Checkconfig(&v, false)
		if err != nil {
			utils.Logger.SugarLogger.Debug(c.confpath, "item:", i, err)
			return
		}
	}

	return 0, nil
}
