package app

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"fmt"
	"github.com/gabriel-vasile/mimetype"
	"github.com/microcosm-cc/bluemonday"
	"github.com/panjf2000/gnet"
	"html"
	"io/ioutil"
	"net"
	"net/http"
	"regexp"
	"strings"
)

/*参考学习
https://github.com/google/martian
https://github.com/snail007/goproxy
https://github.com/AdguardTeam/gomitmproxy
*/

/*防注入
go get github.com/microcosm-cc/bluemonday@v1.0.8
go get github.com/gorilla/csrf
*/

/**功能支持
1】ipv4 p2p代理转换
2】ipv6 p2p代理转换

//todo *表示功能难点和复杂度
1】*ipv4和ipv6的代理转换（over）
2】**http2,https,gmssl算法支持(SM3,SM4)
3】过滤功能
	3.1 数据类型, Content-Type
    3.2 数据大小, Content-Length
	3.3 *病毒检测
	3.4 **URL,header,body内容（过滤,修改or删除）防注入(XSS,SQL...)
	3.5 *资产识别
4】dns域名
5】**http-https代理转换
6】**自动重定向,嵌套深度
*/

type Http struct {
	requ       *http.Request
	resp       *http.Response
	body       []byte
	bodyoffset int

	newout bytes.Buffer
}

var HttpMethods = []string{
	"GET",
	"POST",
	"CONNECT",
	"PUT",
	"HEAD",
	"TRACE",
	"OPTIONS",
	"DELETE",
	"UPDATE",
}

func (a *Http) filter_request() (Filter_t, *http.Request, []byte) {

	strings.HasPrefix(a.requ.URL.Path, "")
	strings.Contains(a.requ.URL.Path, "")
	regexp.MatchString("", a.requ.URL.Path)

	a.requ.URL.EscapedFragment()
	a.requ.URL.EscapedPath()
	a.requ.URL.Query()

	//todo header

	//html sanitize
	if a.requ.Header.Get("Content-Type") == "text/html" {
		p := bluemonday.NewPolicy()
		p.AllowTables()
		p.SanitizeBytes(a.body[:a.bodyoffset])
	}

	return FILTER_ACCEPT, nil, nil
}

func (a *Http) filter_response() (Filter_t, *http.Response, []byte) {

	var tmp http.Response
	var san []byte

	tmp = *a.resp
	tmp.Header.Del("Server")
	tmp.Header.Del("Date")
	tmp.Header.Del("X-Pad") //通常是Apache为避免浏览器BUG生成的头部，默认忽略
	//tmp.Header.Del("X-Accel-*") //用于控制nginx行为的响应，不需要向客户端转发

	if (a.bodyoffset > 0) && (strings.HasPrefix(a.resp.Header.Get("Content-Type"), "text/html")) {
		p := bluemonday.UGCPolicy()
		san = p.SanitizeBytes(a.body[:a.bodyoffset])
		fmt.Println(string(san))
		fmt.Println("sanitize ", a.bodyoffset, len(a.body), len(san))

		//todo 不完整的数据
		tmp.ContentLength = int64(len(san))
	}

	var out bytes.Buffer
	tmp.Write(&out)
	fmt.Println(out.String())
	if san != nil {
		out.Write(san)
	} else if a.bodyoffset > 0 {
		out.Write(a.body[:a.bodyoffset])
	}

	return FILTER_MODIFY, &tmp, out.Bytes()

}

func (a *Http) ParserRequ(packet []byte, c gnet.Conn, p interface{}) (interface{}, []byte, error) {
	var rege string = "^("
	for i, s := range HttpMethods {
		if i > 0 {
			rege += "|" + s
		} else {
			rege += s
		}
	}
	rege += ")"

	if r, _ := regexp.Match(rege, packet); r {
		fmt.Printf("%s\n", string(packet))
		a.requ, _ = http.ReadRequest(bufio.NewReader(bytes.NewReader(packet)))
		fmt.Println(a.requ)
		//s, _ := url.QueryUnescape(a.requ.URL.String())
		if a.requ != nil {
			fmt.Println(a.requ.URL.Path)
			a.resp = nil

			a.body = make([]byte, a.requ.ContentLength)
			a.bodyoffset = 0

			for a.bodyoffset < int(a.requ.ContentLength) {
				m, err := a.requ.Body.Read(a.body[a.bodyoffset:])
				if m > 0 {
					a.bodyoffset += m
				}
				if err != nil {
					fmt.Println(err)
					break
				}
			}

			a.filter_request()

			var tmp http.Request
			tmp = *a.requ
			tmp.Header.Set("User-Agent", "gope/1.0")
			xff := tmp.Header.Get("X-Forwarded-For")
			if len(xff) == 0 {
				xff = strings.Split(c.RemoteAddr().String(), ":")[0] //+ ", " + p.(*Tcp_udp_s).Config.Bindip_str
			} else {
				xff += ", " + strings.Split(c.RemoteAddr().String(), ":")[0]
			}
			tmp.Header.Set("X-Forwarded-For", xff)
			tmp.Host = net.JoinHostPort(p.(*Tcp_udp_s).Config.Upstreamip_str, fmt.Sprint(p.(*Tcp_udp_s).Config.Upstreamport))
			fmt.Println(tmp)

			a.newout.Reset()
			tmp.Write(&a.newout)
			fmt.Println(a.newout.String())
		}
	} else if a.requ != nil {
		copy(a.body[a.bodyoffset:], packet)
		a.bodyoffset += len(packet)
		a.newout.Reset()
	}

	if a.requ != nil {
		fmt.Printf("body size %d:%d\n", a.requ.ContentLength, a.bodyoffset)

		if a.bodyoffset > 0 {
			if strings.HasPrefix(a.requ.Header.Get("Content-Type"), "text") {
				fmt.Println(html.UnescapeString(string(a.body[:a.bodyoffset])))
			} else {

			}

			kind := mimetype.Detect(a.body[:a.bodyoffset])
			fmt.Println(a.requ.Method, "filetype:", kind)
			if kind.Extension() == "" {

			}

			if a.bodyoffset == int(a.requ.ContentLength) {
				//todo 过滤功能
				//filter_body()
			}

		}
	}

	if a.newout.Len() > 0 {
		return nil, a.newout.Bytes(), nil
	}
	return nil, nil, nil
}

func (a *Http) ParserResp(packet []byte, c gnet.Conn, p interface{}) (interface{}, []byte, error) {
	//fmt.Printf("%s\n", string(packet))
	if a.resp == nil {
		a.resp, _ = http.ReadResponse(bufio.NewReader(bytes.NewReader(packet)), a.requ)
		fmt.Println(a.resp)
		if a.resp == nil {
			//return gnet.Close, []byte{}, nil
			return nil, nil, nil
		}
		a.body = make([]byte, a.resp.ContentLength)
		a.bodyoffset = 0

		for a.bodyoffset < int(a.resp.ContentLength) {
			m, err := a.resp.Body.Read(a.body[a.bodyoffset:])
			if m > 0 {
				a.bodyoffset += m
			}
			if err != nil {
				fmt.Println(err)
				break
			}
		}

		if (a.resp.StatusCode >= http.StatusMultipleChoices) && (a.resp.StatusCode <= http.StatusPermanentRedirect) {
			fmt.Println(a.resp.Location())
			//todo 重定向
		}

	} else {
		fmt.Println("body len", len(packet), a.bodyoffset)
		//todo
		copy(a.body[a.bodyoffset:], packet)
		a.bodyoffset += len(packet)
	}

	fmt.Printf("body size %d:%d\n", a.resp.ContentLength, a.bodyoffset)

	if a.bodyoffset > 0 {
		if strings.HasPrefix(a.resp.Header.Get("Content-Type"), "text") {
			fmt.Println(html.UnescapeString(string(a.body[:a.bodyoffset])))
		} else {

		}

		kind := mimetype.Detect(a.body[:a.bodyoffset])
		fmt.Println(a.requ.Method, "filetype:", kind)
		if kind.Extension() == "" {

		}

		if a.bodyoffset == int(a.requ.ContentLength) {

			switch a.resp.Header.Get("Content-Encoding") {
			case "gzip":
				zip, err := gzip.NewReader(bytes.NewReader(a.body))
				if err == nil {
					defer zip.Close()
					unzip, err := ioutil.ReadAll(zip)
					if err == nil {
						fmt.Println(string(unzip))
					}
				}

			case "compress":
			case "deflate":
				zip := flate.NewReader(bytes.NewReader(a.body))
				defer zip.Close()
				unzip, err := ioutil.ReadAll(zip)
				if err == nil {
					fmt.Println(string(unzip))
				}
			case "identiy":
			case "br":
			default:

			}
		}

	}

	if f, _, o := a.filter_response(); f == FILTER_MODIFY {
		return nil, o, nil
	}

	return nil, nil, nil
}

func (a *Http) Reset(c gnet.Conn) {
}

func (a *Http) Tick(parent interface{}) {

}

func (a *Http) Startup(parent interface{}) error {
	return nil
}
