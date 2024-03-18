package app

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/gabriel-vasile/mimetype"
	"github.com/panjf2000/gnet"
	"html"
	"net"
	"net/http"
	"regexp"
	"strings"
)

type Http struct {
	requ       *http.Request
	resp       *http.Response
	body       []byte
	bodyoffset int

	newout bytes.Buffer
}

var methods = []string{
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

	//todo header

	return FILTER_ACCEPT, nil, nil
}

func (a *Http) filter_response() (Filter_t, *http.Response, []byte) {

	var tmp http.Response
	tmp = *a.resp
	tmp.Header.Del("Server")
	tmp.Header.Del("Date")
	tmp.Header.Del("X-Pad") //通常是Apache为避免浏览器BUG生成的头部，默认忽略
	//tmp.Header.Del("X-Accel-*") //用于控制nginx行为的响应，不需要向客户端转发

	var out bytes.Buffer
	tmp.Write(&out)
	fmt.Println(out.String())

	return FILTER_MODIFY, &tmp, out.Bytes()

}

func (a *Http) ParserRequ(packet []byte, c gnet.Conn, p interface{}) (interface{}, []byte, error) {
	var rege string = "^("
	for i, s := range methods {
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

			//filter_request()

			var tmp http.Request
			tmp = *a.requ
			tmp.Header.Set("User-Agent", "gope/1.0")
			xff := tmp.Header.Get("X-Forwarded-For")
			if len(xff) == 0 {
				xff = strings.Split(c.RemoteAddr().String(), ":")[0] + ", " + p.(*Tcp_udp_s).Config.Bindip_str
			} else {
				xff += ", " + p.(*Tcp_udp_s).Config.Bindip_str
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
			if strings.HasPrefix(a.requ.Header["Content-Type"][0], "text") {
				fmt.Println(html.UnescapeString(string(a.body)))
			} else {

			}

			kind := mimetype.Detect(a.body)
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

	} else {
		fmt.Println("body len", len(packet), a.bodyoffset)
		//todo
		copy(a.body[a.bodyoffset:], packet)
		a.bodyoffset += len(packet)
	}

	fmt.Printf("body size %d:%d\n", a.resp.ContentLength, a.bodyoffset)

	if a.bodyoffset > 0 {
		if strings.HasPrefix(a.resp.Header["Content-Type"][0], "text") {
			fmt.Println(html.UnescapeString(string(a.body)))
		} else {

		}

		kind := mimetype.Detect(a.body)
		fmt.Println(a.requ.Method, "filetype:", kind)
		if kind.Extension() == "" {

		}

		if a.bodyoffset == int(a.requ.ContentLength) {

		}

	}

	if f, _, o := a.filter_response(); f == FILTER_MODIFY {
		if a.bodyoffset > 0 {
			o = append(o, a.body...)
		}
		return nil, o, nil
	}

	return nil, nil, nil
}

func (a *Http) Reset(c gnet.Conn) {
}

func (a *Http) Tick(parent interface{}) {
}
