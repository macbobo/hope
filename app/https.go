package app

import (
	"bytes"
	"crypto/tls"
	"encoding/pem"
	"fmt"
	"github.com/macbobo/gope/app/tls_api"
	"github.com/microcosm-cc/bluemonday"
	"github.com/panjf2000/gnet"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

type Https struct {
	Http
	tlsversion string
	tlscert    string //证书配置
	tlskey     string
	mitmstate  map[string]int //会话状态
}

func (a *Https) ParserRequ(packet []byte, c gnet.Conn, p interface{}) (interface{}, []byte, error) {
	tlsi := tls_api.ParseTLSPayload(packet)
	fmt.Println("client", tlsi)

	//todo MITM劫持
	k := c.Context()
	switch a.mitmstate[k.(string)] {
	case 0:
		if tls_api.GetTLSVersion(tlsi.ClientHelloTLSRecord.HandshakeProtocol.TLSVersion) != "" {
			//cert, err := tls.LoadX509KeyPair(a.tlscert, a.tlskey)
			//if err == nil {
			//	cfg := &tls.Config{Certificates: []tls.Certificate{cert}}
			//}
		}
	case 1:
	case 2:
	default:

	}

	return nil, nil, nil
}

func (a *Https) ParserResp(packet []byte, c gnet.Conn, p interface{}) (interface{}, []byte, error) {
	tls := tls_api.ParseTLSPayload(packet)
	fmt.Println("server", tls)

	return nil, nil, nil
}

func (a *Https) Reset(c gnet.Conn) {
	a.Http.Reset(c)
}

func (a *Https) Tick(parent interface{}) {
	a.Http.Tick(parent)
}

func (a *Https) Startup(parent interface{}) error {
	/*
		# 生成私钥
		openssl genpkey -algorithm RSA -out server.key
		# 生成自签名的服务器证书请求
		openssl req -new -key server.key -out server.csr
		# 生成自签名的服务器证书
		openssl x509 -req -in server.csr -signkey server.key -out server.crt
	*/

	m := parent.(*Tcp_udp_s)

	//cert, err := tls.LoadX509KeyPair(m.app.(*Https).tlscert, m.app.(*Https).tlskey)
	cert, err := tls.LoadX509KeyPair("./test/server.crt", "./test/server.key")
	if err != nil {
		return err
	}

	https := http.Server{
		Addr: net.JoinHostPort(m.Config.Bindip_str, fmt.Sprint(m.Config.Bindport)),
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
		},
		ConnState: func(conn net.Conn, state http.ConnState) {
			fmt.Println(conn.LocalAddr(), conn.RemoteAddr(), state)
			switch state {
			case http.StateNew:
			case http.StateClosed:
			case http.StateActive:

			default:

			}

		},
		//ConnContext: func(ctx context.Context, c net.Conn) context.Context {
		//
		//	return nil
		//},
	}

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
				response.Body.Close()
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

			oldDirector(request)
			request.Header.Set("User-Agent", "gope/1.0")
			request.Proto = "HTTP/1.1"
			request.ProtoMajor, request.ProtoMinor, _ = http.ParseHTTPVersion(request.Proto)
			//request.Host

			fmt.Println(request)

		}

		ps.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, err error) {
			fmt.Println(err)
		}

		ps.ServeHTTP(writer, request)

	})

	err = https.ListenAndServeTLS("", "")
	if err != nil {
		fmt.Println(err)

	}
	return err
}
