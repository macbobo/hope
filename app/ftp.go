package app

import (
	"fmt"
	"github.com/panjf2000/gnet"
)

/**
功能支持
1】ipv4 p2p代理转换 PASV/PORT
2】ipv6 p2p代理转换 EPSV

//todo *表示功能难点和复杂度
1】*ipv4和ipv6的代理转换（over）
2】ipv6协议完善
3】命令字典解析扩展
4】过滤功能
	4.1 命令字
    4.2 文件路径/文件名 <demo ok>
	4.3 *文件类型 <demo ok>
    4.4 文件大小
	4.5 *病毒检测
	4.6 **文件内容（过滤，修改or删除） <demo ok:修改or删除暂且为纯文本>
*/

type Ftp struct {
	line string
	ctrl Ftpcmd
	data *Ftpdata
}

func (a *Ftp) ParserRequ(packet []byte, c gnet.Conn, parent interface{}) (interface{}, []byte, error) {
	a.line += string(packet)
	if packet[len(packet)-1] == '\n' {
		a.ctrl.Check(a.line)

		defer func() {
			a.line = ""
		}()

		switch a.ctrl.cmd {
		case "PORT":
			k := c.Context()
			cm, _ := parent.(*Tcp_udp_s).conntracks.Load(k)
			if cm != nil {
				return ftp4PrePORT(a, cm.(*conntrack_t), c, parent)
			}
		default:

		}

		if r, err := a.ctrl.filter(); r != FILTER_ACCEPT {
			return nil, []byte{}, err
		}
	}
	return nil, nil, nil
}

func (a *Ftp) ParserResp(packet []byte, c gnet.Conn, parent interface{}) (interface{}, []byte, error) {
	a.line += string(packet)
	if packet[len(packet)-1] == '\n' {
		fmt.Println(string(packet))

		defer func() {
			a.line = ""
		}()

		if a.ctrl.Setret(string(a.line)) {

			//动态端口命令
			switch a.ctrl.cmd {
			case "PASV":
				k := c.Context()
				cm, _ := parent.(*Tcp_udp_c).conntracks.Load(k)
				if cm != nil {
					return ftp4PASV(a, cm.(*conntrack_t), c, parent)
				}
			case "PORT":
				k := c.Context()
				cm, _ := parent.(*Tcp_udp_c).conntracks.Load(k)
				if cm != nil {
					return ftp4PORT(a, cm.(*conntrack_t), c, parent)
				}
			case "EPSV":
				k := c.Context()
				cm, _ := parent.(*Tcp_udp_c).conntracks.Load(k)
				if cm != nil {
					return ftp6PASV(a, cm.(*conntrack_t), c, parent)
				}
			case "EPRT":
			default:

			}
		}
	}
	return nil, nil, nil
}

func (a *Ftp) Reset(c gnet.Conn) {
	a.line = ""
	a.ctrl.Clear()
	if a.data != nil {
		a.data.session = nil
		a.data = nil
	}
}

func (a *Ftp) Tick(parent interface{}) {
	fmt.Println(parent.(*Tcp_udp_c).Config.Subext)
	//todo 保留最近10个子连接记录
	all := len(parent.(*Tcp_udp_c).Config.Subext)
	if all > 10 {
		parent.(*Tcp_udp_c).Config.Subext = parent.(*Tcp_udp_c).Config.Subext[all-10:]
	}
}

func (a *Ftp) Startup(parent interface{}) error {
	return nil
}
