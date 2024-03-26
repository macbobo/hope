package app

import (
	"bytes"
	"crypto/tls"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/macbobo/gope/config"
	"github.com/macbobo/gope/utils"
	"github.com/microcosm-cc/bluemonday"
	"github.com/panjf2000/ants/v2"
	"github.com/panjf2000/gnet"
	"github.com/panjf2000/gnet/pkg/logging"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
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
	uid         string
	key         string
	src         gnet.Conn
	dst         gnet.Conn
	dst_cli     *gnet.Client
	active      time.Time
	src_pacekts [2]int64
	src_bytes   [2]int64
	dst_pacekts [2]int64
	dst_bytes   [2]int64

	user interface{}
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
	var cmi *conntrack_t

	if cm != nil {
		cmi = cm.(*conntrack_t)
		cmi.dst_pacekts[0] += 1
		cmi.dst_bytes[0] += int64(len(packet))
		cmi.active = time.Now()
	}

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
			if m.gpool != nil {
				m.gpool.Submit(func() {
					cmi.src.AsyncWrite(t)
				})
			} else {
				gwrite(cmi.src, t)
			}

		} else {
			cmi.src.SendTo(packet)
		}
		cmi.src_pacekts[1] += 1
		cmi.src_bytes[1] += int64(len(packet))
	}
	return
}

func (m *Tcp_udp_c) OnClosed(c gnet.Conn, err error) (action gnet.Action) {

	k := c.Context()

	if m.app != nil {
		m.app.Reset(c)
	}

	if c.BufferLength() > 0 {
		fmt.Println("warning")
	}
	d := c.Read()
	if len(d) > 0 {
		fmt.Println("warning")
	}

	cm, _ := m.conntracks.Load(k)
	if cm != nil {
		fmt.Println("test close3", time.Now(), cm.(*conntrack_t))

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
			ts := "@" + time.Now().String()
			k1 := fmt.Sprintf("%s://%s->%s", m.Config.Protocol_str, c.LocalAddr(), c.RemoteAddr())
			v1 := new(conntrack_t)
			*v1 = conntrack_t{src: c, dst: pc, active: time.Now(), uid: k1 + ts, key: k1, dst_cli: nil}
			m.conntracks.Store(k1, v1)
			c.SetContext(k1)

			//key must SetContext
			k2 := fmt.Sprintf("%s://%s->%s", m.Config.Protocolex_str, pc.LocalAddr(), pc.RemoteAddr())
			v2 := new(conntrack_t)
			*v2 = conntrack_t{src: c, dst: pc, active: time.Now(), uid: k1 + ts, key: k2, dst_cli: p}
			m.conntracks.Store(k2, v2)
		}
	}

	return
}

func (m *Tcp_udp_s) React(packet []byte, c gnet.Conn) (out []byte, action gnet.Action) {

	k := c.Context()

	cm, _ := m.conntracks.Load(c.Context())
	var cmi *conntrack_t
	if cm != nil {
		cmi = cm.(*conntrack_t)
		cmi.src_pacekts[0] += 1
		cmi.src_bytes[0] += int64(len(packet))
		cm.(*conntrack_t).active = time.Now()
	}

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

	if cm != nil {
		if m.Config.Protocolex == config.PROTOCOL_TCP {
			var t []byte
			t = append(t, packet...)
			if m.gpool != nil {
				m.gpool.Submit(func() {
					cmi.dst.AsyncWrite(t)
				})
			} else {
				gwrite(cmi.dst, t)
			}

		} else {
			cmi.dst.SendTo(packet)
		}
		cmi.dst_pacekts[1] += 1
		cmi.dst_bytes[1] += int64(len(packet))
	} else if m.Config.Protocol == config.PROTOCOL_UDP {
		m.OnOpened(c)
		cm, _ = m.conntracks.Load(k)
		if cm != nil {
			cmi = cm.(*conntrack_t)
			cmi.src_pacekts[0] += 1
			cmi.src_bytes[0] += int64(len(packet))
			cm.(*conntrack_t).active = time.Now()

			if m.Config.Protocolex == config.PROTOCOL_TCP {
				var t []byte
				t = append(t, packet...)
				if m.gpool != nil {
					m.gpool.Submit(func() {
						cmi.dst.AsyncWrite(t)
					})
				} else {
					gwrite(cmi.dst, t)
				}

			} else {
				cmi.dst.SendTo(packet)
			}
			cmi.dst_pacekts[1] += 1
			cmi.dst_bytes[1] += int64(len(packet))
		}

	}
	return
}

func (m *Tcp_udp_s) OnClosed(c gnet.Conn, err error) (action gnet.Action) {
	k := c.Context()

	if m.app != nil {
		m.app.Reset(c)
	}
	if c.BufferLength() > 0 {
		fmt.Println("warning")
	}

	cm, _ := m.conntracks.Load(k)
	if cm != nil {
		fmt.Println("test close1", time.Now(), cm.(*conntrack_t))
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
	case "https":
		m.app = new(Https)
	default:
		return -1, errors.New(fmt.Sprintf("app<%s> unknown", m.Config.App))

	}
	return 0, nil
}
func (m *Tcp_udp_s) Wait() {

	m.wg.Wait()
	if m.gpool != nil {
		m.gpool.Release()
	}
}

func (m *Tcp_udp_s) Startup() (int, error) {

	m.wg.Add(1)

	go func() {
		if m.Config.App == "https" {
			/*
				# 生成私钥
				openssl genpkey -algorithm RSA -out server.key
				# 生成自签名的服务器证书请求
				openssl req -new -key server.key -out server.csr
				# 生成自签名的服务器证书
				openssl x509 -req -in server.csr -signkey server.key -out server.crt
			*/

			//cert, err := tls.LoadX509KeyPair(m.app.(*Https).tlscert, m.app.(*Https).tlskey)
			cert, err := tls.LoadX509KeyPair("./test/server.crt", "./test/server.key")
			if err == nil {
				https := http.Server{Addr: net.JoinHostPort(m.Config.Bindip_str, fmt.Sprint(m.Config.Bindport)),
					TLSConfig: &tls.Config{
						Certificates: []tls.Certificate{cert},
						MinVersion:   tls.VersionTLS12,

						//todo 算法概念？
						//ClientAuth:   tls.RequireAndVerifyClientCert,
						//CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
						//PreferServerCipherSuites: true,
						//CipherSuites: []uint16{
						//	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
						//	tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
						//	tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
						//	tls.TLS_RSA_WITH_AES_256_CBC_SHA,
						//
						//},
					}}

				http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
					fmt.Println(request)
					if request.TLS != nil {
						//dump, _ := httputil.DumpRequest(request, true)
						//fmt.Printf("%q", dump)
						//fmt.Println()

						cert_c := request.TLS.PeerCertificates
						if len(cert_c) > 0 {
							// 将证书切片转换为字符串
							certStr := ""
							for _, c := range cert_c {
								certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})
								certStr += string(certPEM)
							}
							request.Header.Set("X-Client-Cert", certStr)
						}
					}

					us, _ := url.Parse(m.Config.Appex + "://" +
						net.JoinHostPort(m.Config.Upstreamip_str, fmt.Sprintf("%d", m.Config.Upstreamport)))
					ps := httputil.NewSingleHostReverseProxy(us)
					ps.ModifyResponse = func(response *http.Response) error {
						//fmt.Println(response.Body)

						// todo 此处理内部是由modifyResponse调用2次
						// 1. statuscode=101
						// 2. 默认调用

						response.Header.Del("Server")
						response.Header.Del("Date")
						if strings.HasPrefix(response.Header.Get("Content-Type"), "text/html") {
							buf, _ := io.ReadAll(response.Body)
							fmt.Println(string(buf))

							p := bluemonday.UGCPolicy()
							san := p.SanitizeBytes(buf)
							fmt.Println(string(san))

							//todo 不完整的数据
							response.ContentLength = int64(len(san))
							//返回新数据
							response.Body = ioutil.NopCloser(bytes.NewReader(san))
						}
						fmt.Println(response.Header)
						return nil
					}
					
					ps.Transport = &http.Transport{
						TLSClientConfig: &tls.Config{Certificates: []tls.Certificate{cert},
							InsecureSkipVerify: true,
							MinVersion:         tls.VersionTLS12,
						},
						//Proxy: func(r *http.Request) (*url.URL, error) {
						//	us, _ := url.Parse(m.Config.App + "://" +
						//		net.JoinHostPort(m.Config.Upstreamip_str, fmt.Sprintf("%d", m.Config.Upstreamport)) + "/su-uos")
						//
						//	return us, nil
						//},
						//GetProxyConnectHeader: func(ctx context.Context, proxyURL *url.URL, target string) (http.Header, error) {
						//	return nil, nil
						//
						//},
						ForceAttemptHTTP2: false,
					}

					oldDirector := ps.Director
					ps.Director = func(request *http.Request) {
						//request是代理转发的请求outreq
						//todo hop-by-hop connection的理解？
						request.Header.Set("User-Agent", "gope/1.0")
						request.Proto = "HTTP/1.0"
						request.ProtoMajor, request.ProtoMinor, _ = http.ParseHTTPVersion(request.Proto)

						oldDirector(request)

						fmt.Println(request)

					}

					ps.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, err error) {
						fmt.Println(err)
					}

					ps.ServeHTTP(writer, request)

				})
				err := https.ListenAndServeTLS("", "")
				if err != nil {
					fmt.Println(err)
				}

			}

		} else {
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
		}
		m.wg.Done()
	}()

	return 0, nil
}
