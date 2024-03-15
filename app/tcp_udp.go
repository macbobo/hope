package app

import (
	"errors"
	"fmt"
	"github.com/macbobo/gope/config"
	"github.com/macbobo/gope/utils"
	"github.com/panjf2000/ants/v2"
	"github.com/panjf2000/gnet"
	"github.com/panjf2000/gnet/pkg/logging"
	"net"
	"sync"
	"time"
)

type App interface {
	ParserRequ(packet []byte, c gnet.Conn, p interface{}) (interface{}, []byte, error) //p服务端根对象
	ParserResp(packet []byte, c gnet.Conn, p interface{}) (interface{}, []byte, error) //p客户端根对象
	Reset(c gnet.Conn)
	Tick(p interface{}) //p客户端根对象
}

type conntrack_t struct {
	ukey    string
	src     gnet.Conn
	dst     gnet.Conn
	dst_cli *gnet.Client
	active  time.Time
}
type Server_t struct {
	Config     config.Config_t
	conntracks *sync.Map
	gpool      *ants.Pool

	app App
}

type Client_t struct {
	Config     config.Config_t
	conntracks *sync.Map
	gpool      *ants.Pool

	app App
}

type Tcp_udp_s struct {
	*gnet.EventServer
	Server_t

	wg sync.WaitGroup
}

type Tcp_udp_c struct {
	*gnet.EventServer
	Client_t
}

func gwrite(conn gnet.Conn, packet []byte) {
	conn.AsyncWrite(packet)
}
func (m *Tcp_udp_c) OnOpened(c gnet.Conn) (out []byte, action gnet.Action) {
	utils.Logger.SugarLogger.Debug(c, c.LocalAddr(), c.RemoteAddr())
	c.SetContext(fmt.Sprintf("%s://%s->%s", m.Config.Protocolex_str, c.LocalAddr(), c.RemoteAddr()))

	return
}

func (m *Tcp_udp_c) AfterWrite(c gnet.Conn, b []byte) {

}

func (m *Tcp_udp_c) React(packet []byte, c gnet.Conn) (out []byte, action gnet.Action) {
	cm, _ := m.conntracks.Load(c.Context())

	if m.app != nil {
		nac, o, err := m.app.ParserResp(packet, c, m)
		if o != nil {
			if len(o) == 0 {
				if nac != nil {
					action = nac.(gnet.Action)
				}
				if err != nil {
					out = []byte(err.Error())
				}
				return
			}
			packet = o
		}
	}

	if cm != nil {
		if m.Config.Protocol == config.PROTOCOL_TCP {
			var t []byte
			t = append(t, packet...)
			cmi := cm.(*conntrack_t)
			if m.gpool != nil {
				m.gpool.Submit(func() {
					cmi.src.AsyncWrite(t)
					cmi.active = time.Now()
				})
			} else {
				gwrite(cmi.src, t)
				cmi.active = time.Now()
			}

		} else {
			cm.(*conntrack_t).src.SendTo(packet)
			cm.(*conntrack_t).active = time.Now()
		}
	}
	return
}

func (m *Tcp_udp_c) OnClosed(c gnet.Conn, err error) (action gnet.Action) {

	k := c.Context()

	d := c.Read()

	if m.app != nil {
		m.app.Reset(c)
	}

	fmt.Println("test close3", time.Now())
	if len(d) > 0 {
		cm, _ := m.conntracks.Load(k)
		if cm != nil {
			if m.Config.Protocol == config.PROTOCOL_TCP {
				cmi := cm.(*conntrack_t)
				if m.gpool != nil {
					m.gpool.Submit(func() {
						cmi.src.AsyncWrite(d)
						cmi.active = time.Now()
					})
				} else {
					gwrite(cmi.src, d)
					cmi.active = time.Now()
				}
			} else {
				cm.(*conntrack_t).src.SendTo(c.Read())
				cm.(*conntrack_t).active = time.Now()
			}
		}
	}

	cm, _ := m.conntracks.Load(k)
	if cm != nil {
		go func() {
			//time.Sleep(time.Millisecond * 1000)
			cm.(*conntrack_t).src.Close()
		}()
		go func() {
			//time.Sleep(time.Millisecond * 100)
			cm.(*conntrack_t).dst_cli.Stop() //Reactor回调是在gorouting的cli.el.run中，不能用同步方式调用，会导致阻塞
		}()
		m.conntracks.Delete(k)
	}

	return gnet.Close //gnet.Shutdown //退出Reactor回调，但是还需要Stop操作，才会释放gorouting资源
}

func (m *Tcp_udp_c) OnShutdown(server gnet.Server) {
	//fmt.Println(server)
}

func (m *Tcp_udp_c) Tick() (delay time.Duration, action gnet.Action) {

	delay = time.Second * 60
	//action = gnet.Shutdown
	if m.app != nil {
		m.app.Tick(m)
	}
	return
}

func (m *Tcp_udp_s) OnInitComplete(server gnet.Server) (action gnet.Action) {

	return
}

func (m *Tcp_udp_s) OnOpened(c gnet.Conn) (out []byte, action gnet.Action) {

	utils.Logger.SugarLogger.Debug(c, c.LocalAddr(), c.RemoteAddr())
	t := new(Tcp_udp_c)
	t.Config = m.Config
	t.conntracks = m.conntracks
	t.gpool = m.gpool
	t.app = m.app

	//todo 当前版本logger->lumberjack在关闭shutdown时有gorouting泄漏？？
	p, err := gnet.NewClient(t,
		gnet.WithTCPKeepAlive(60*time.Second),
		gnet.WithTicker(true), //必须实现接口Ticker，否则不主动Stop，cpu占用高
		//gnet.WithLockOSThread(true),
		gnet.WithLogPath("./gnet_c.log"),
		gnet.WithLogLevel(logging.DebugLevel),
		gnet.WithSocketRecvBuffer(1024*1024),
	)
	if err != nil {
		utils.Logger.SugarLogger.Error(err)
		c.Close()
		action = gnet.Close
	} else {
		pc, err := p.Dial(m.Config.Protocolex_str,
			net.JoinHostPort(m.Config.Upstreamip_str, fmt.Sprint(m.Config.Upstreamport)))
		if err != nil {

			utils.Logger.SugarLogger.Error(err)
			c.Close()
			p.Stop()
			action = gnet.Close
		} else {
			p.Start()
			k := fmt.Sprintf("%s://%s->%s", m.Config.Protocol_str, c.LocalAddr(), c.RemoteAddr())
			v := new(conntrack_t)
			*v = conntrack_t{src: c, dst: pc, active: time.Now(), ukey: time.Now().String(), dst_cli: nil}
			m.conntracks.Store(k, v)
			c.SetContext(k)

			k = fmt.Sprintf("%s://%s->%s", m.Config.Protocolex_str, pc.LocalAddr(), pc.RemoteAddr())
			v = new(conntrack_t)
			*v = conntrack_t{src: c, dst: pc, active: time.Now(), ukey: time.Now().String(), dst_cli: p}
			m.conntracks.Store(k, v)
		}
	}

	return
}

func (m *Tcp_udp_s) React(packet []byte, c gnet.Conn) (out []byte, action gnet.Action) {

	k := c.Context()

	if m.app != nil {
		nac, o, err := m.app.ParserRequ(packet, c, m)
		if o != nil {
			if len(o) == 0 {
				if nac != nil {
					action = nac.(gnet.Action)
				}
				if err != nil {
					out = []byte(err.Error())
				}
				return
			}

			packet = o
		}

	}

	cm, _ := m.conntracks.Load(c.Context())
	if cm != nil {
		if m.Config.Protocolex == config.PROTOCOL_TCP {
			var t []byte
			t = append(t, packet...)
			if m.gpool != nil {
				m.gpool.Submit(func() {
					cm.(*conntrack_t).dst.AsyncWrite(t)
					cm.(*conntrack_t).active = time.Now()
				})
			} else {
				gwrite(cm.(*conntrack_t).dst, t)
				cm.(*conntrack_t).active = time.Now()
			}

		} else {
			cm.(*conntrack_t).dst.SendTo(packet)
			cm.(*conntrack_t).active = time.Now()
		}
	} else if m.Config.Protocol == config.PROTOCOL_UDP {
		m.OnOpened(c)
		cm, _ = m.conntracks.Load(k)
		if cm != nil {
			if m.Config.Protocolex == config.PROTOCOL_TCP {
				var t []byte
				t = append(t, packet...)
				if m.gpool != nil {
					m.gpool.Submit(func() {
						cm.(*conntrack_t).dst.AsyncWrite(t)
						cm.(*conntrack_t).active = time.Now()
					})
				} else {
					gwrite(cm.(*conntrack_t).dst, t)
					cm.(*conntrack_t).active = time.Now()
				}

			} else {
				cm.(*conntrack_t).dst.SendTo(packet)
				cm.(*conntrack_t).active = time.Now()
			}
		}

	}
	return
}

func (m *Tcp_udp_s) OnClosed(c gnet.Conn, err error) (action gnet.Action) {
	k := c.Context()

	if m.app != nil {
		m.app.Reset(c)
	}

	fmt.Println("test close1")
	cm, _ := m.conntracks.Load(k)
	if cm != nil {
		cm.(*conntrack_t).dst.Close()
		m.conntracks.Delete(k)
	}
	//time.Sleep(time.Microsecond * 1000)

	return gnet.Close
}

func (m *Tcp_udp_s) Tick() (delay time.Duration, action gnet.Action) {

	delay = time.Second * 60
	//action = gnet.Shutdown
	m.conntracks.Range(func(key, value interface{}) bool {
		if (value.(*conntrack_t).active.Unix() + 60) < time.Now().Unix() {
			utils.Logger.SugarLogger.Debug("time gc", key, value)
			value.(*conntrack_t).src.Close()
			value.(*conntrack_t).dst.Close()
			if value.(*conntrack_t).dst_cli != nil {
				go value.(*conntrack_t).dst_cli.Stop()
			}
			m.conntracks.Delete(key)
		}

		return true
	})
	return
}

func (m *Tcp_udp_s) Init() (int, error) {

	m.conntracks = new(sync.Map)
	//m.gpool, _ = ants.NewPool(1000, ants.WithNonblocking(true))
	//p := goroutine.Default()

	switch m.Config.App {
	case "":
	case "ftp":
		m.app = new(Ftp)
	case "http":
		m.app = new(Http)
	default:
		return -1, errors.New(fmt.Sprintf("app<%s> unknown", m.Config.App))

	}
	return 0, nil
}
func (m *Tcp_udp_s) Wait() {

	m.wg.Wait()
	m.gpool.Release()
}

func (m *Tcp_udp_s) Startup() (int, error) {

	m.wg.Add(1)

	go func() {
		err := gnet.Serve(m,
			fmt.Sprintf("%s://%s", m.Config.Protocol_str, net.JoinHostPort(m.Config.Bindip_str, fmt.Sprint(m.Config.Bindport))),
			gnet.WithMulticore(true),
			gnet.WithTicker(true),
			gnet.WithReusePort(true),
			gnet.WithTCPKeepAlive(60*time.Second),
			gnet.WithLogPath("./gnet_s.log"),
			gnet.WithLogLevel(logging.DebugLevel),
		)

		if err != nil {
			utils.Logger.SugarLogger.Error(err)
		}
		m.wg.Done()
	}()

	return 0, nil
}
