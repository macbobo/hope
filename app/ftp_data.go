package app

import (
	"errors"
	"fmt"
	"github.com/gabriel-vasile/mimetype"
	"github.com/h2non/filetype"
	"github.com/macbobo/gope/config"
	"github.com/macbobo/gope/utils"
	"github.com/panjf2000/gnet"
	"github.com/panjf2000/gnet/pkg/logging"
	"github.com/unidoc/unioffice/document"
	"github.com/unidoc/unioffice/presentation"
	"github.com/unidoc/unioffice/spreadsheet"
	"golang.org/x/text/encoding/simplifiedchinese"
	"math/rand"
	"net"
	"os"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	FTPDATA_PASV int = iota
	FTPDATA_PORT
)

const (
	CODE_UNKNOWN int = iota - 1
	CODE_UTF8
	CODE_GBK
)

type Ftpdata struct {
	gnet.EventServer
	Server_t

	onceopened bool
	active     time.Time

	//
	mode      int //0:PASV 1:PORT
	session   *Ftp
	filecache []byte
	cntcahce  int

	//异步文件内容过滤
	filetype string //文件类型
	txtcode  int
	iostream chan []byte
	ioend    chan bool
	fwcache  []byte //大文件流分段回写，过滤后文件不完整
}

//实现由于创建数据连接的客户端是Tcp_udp_c定义app接口
type Ftpdata_app struct {
	session *Ftp
}

func (a *Ftpdata_app) ParserRequ(packet []byte, c gnet.Conn, p interface{}) (interface{}, []byte, error) {

	return a.ParserResp(packet, c, p)
}
func (a *Ftpdata_app) ParserResp(packet []byte, c gnet.Conn, p interface{}) (interface{}, []byte, error) {

	defer func() {
		if recover() != nil {
			utils.Logger.SugarLogger.Panic(string(debug.Stack()))
			debug.PrintStack()
		}
	}()

	fmt.Println("Ftpdata_app client")

	//todo 是否过滤
	if a.session.data.filter_datatype(packet) == FILTER_DROP {

		return gnet.Close, []byte{}, nil

	} else {
		r, o, _ := a.session.data.filter_list(packet)
		switch r {
		case FILTER_DROP:
			return gnet.Close, []byte{}, nil
		case FILTER_CACHE:
			fallthrough
		case FILTER_MODIFY:
			if len(o) == 0 {
				return nil, []byte{}, nil
			}

			//cm, _ := a.session.data.conntracks.Load(c.Context())
			//cmi := cm.(*conntrack_t)
			//if a.session.data.gpool != nil {
			//	a.session.data.gpool.Submit(func() {
			//		cmi.src.AsyncWrite(a.session.data.filecache)
			//		cmi.active = time.Now()
			//		//a.session.data.filecache = []byte{}
			//	})
			//} else {
			//	gwrite(cmi.src, a.session.data.filecache)
			//	cmi.active = time.Now()
			//	//a.session.data.filecache = []byte{}
			//}
			return nil, o, nil
		default:

		}

		r, o = a.session.data.filter_data(packet)
		switch r {
		case FILTER_DROP:
			return gnet.Close, []byte{}, nil
		case FILTER_CACHE:
			fallthrough
		case FILTER_MODIFY:
			if len(o) == 0 {
				return nil, []byte{}, nil
			}
			return nil, o, nil
		default:
		}
	}

	return nil, nil, nil
}
func (a *Ftpdata_app) Reset(c gnet.Conn) {
	m := a.session.data

	if m.iostream != nil {
		fmt.Println("test end1")
		m.iostream <- []byte{}

		select {
		case <-m.ioend:
			o := <-m.iostream
			if len(o) > 0 {
				cm, _ := m.conntracks.Load(c.Context())
				cmi := cm.(*conntrack_t)
				if m.gpool != nil {
					m.gpool.Submit(func() {
						cmi.dst.AsyncWrite(o)
						cmi.active = time.Now()
					})
				} else {

					fmt.Println("test write", time.Now())
					gwrite(cmi.dst, o)
					//res := splitBytes(o, 16*1024)
					//for _, e := range res {
					//	gwrite(cmi.dst, e)
					//	runtime.Gosched()
					//}
					cmi.active = time.Now()
				}
			}
		case <-time.After(time.Millisecond * 100000): //todo 异步处理时间优化？
			fmt.Println("timeout")
		}

		close(m.ioend)
		close(m.iostream)
		m.iostream = nil
	}

	a.session = nil

}
func (a *Ftpdata_app) Tick(p interface{}) {

}
func (m *Ftpdata) OnOpened(c gnet.Conn) (out []byte, action gnet.Action) {

	if m.onceopened {
		action = gnet.Close
	} else {
		m.onceopened = true

		t := new(Tcp_udp_c)
		t.Config = m.Config
		t.conntracks = m.conntracks
		t.gpool = m.gpool
		t.app = new(Ftpdata_app) //m.app
		t.app.(*Ftpdata_app).session = m.session

		//todo 当前版本logger->lumberjack在关闭shutdown时有gorouting泄漏？？
		p, err := gnet.NewClient(t,
			gnet.WithTCPKeepAlive(5*time.Second),
			//gnet.WithTicker(true), //必须实现接口Ticker，否则不主动Stop，cpu占用高
			//gnet.WithLockOSThread(true),
			gnet.WithLogPath("./gnet_c.log"),
			gnet.WithLogLevel(logging.DebugLevel),
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
				action = gnet.Shutdown
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
	}

	return
}

func (m *Ftpdata) filter_list(packet []byte) (Filter_t, []byte, error) {
	if (m.session.ctrl.cmd == "LIST") && (len(m.filecache) < 16*1024*1024) { //10W-50W文件
		m.filecache = append(m.filecache, packet...)
		m.cntcahce++

		//文件名
		if m.filecache[len(m.filecache)-1] == '\n' {
			fmt.Println(m.session.ctrl, string(m.filecache))

			if utf8.Valid(m.filecache) {
				fmt.Println(m.session.ctrl, string(m.filecache))
				rege := regexp.MustCompile(`[d-].*\n`)
				flist := rege.FindAllString(string(m.filecache), -1)
				//flistn := rege.FindAllStringIndex(string(m.filecache), -1)

				m.filecache = []byte{}
				rege = regexp.MustCompile(`\S+`)
				for _, l := range flist {
					fm := rege.FindAllString(l, -1)
					fname := fm[len(fm)-1]
					//过滤文件，i不区分大小写
					if match, _ := regexp.MatchString(`(?i)\.txt|\.tmp$`, fname); !match {
						m.filecache = append(m.filecache, []byte(l)...)
					}
				}

			} else {
			}
			return FILTER_MODIFY, m.filecache, nil
		} else {
			//临时缓冲数据
			fmt.Println("ftp data caching...")
			return FILTER_CACHE, []byte{}, nil
		}
	} else if (m.session.ctrl.cmd == "LIST") && (len(m.filecache) >= 16*1024*1024) {
		////todo 缓冲溢出处理
		m.filecache = append(m.filecache, packet...)
		return FILTER_MODIFY, m.filecache, errors.New("cache overflow")
	}

	return FILTER_ACCEPT, nil, nil
}

func (m *Ftpdata) filter_datatype(packet []byte) Filter_t {

	if (m.session.ctrl.cmd != "LIST") && (len(m.filecache) < 10*1024) { //10KB是依据了filetype库源码配置
		m.filecache = append(m.filecache, packet...)
		m.cntcahce++

		//文件类型
		kind1, _ := filetype.Match(m.filecache)
		fmt.Println(m.session.ctrl, kind1, "filetype:", kind1.Extension)

		//详情查看包文件supported_mimes.md
		//推荐使用，支持识别纯文本内容
		kind2 := mimetype.Detect(m.filecache)
		fmt.Println(m.session.ctrl, kind2, "filetype:", kind2.Extension())

		if (kind1.Extension == "unknown") && (kind2.Extension() == "") {
			//binary octet-stream (.bin)
			m.filetype = ""
		}

		m.filetype = kind2.Extension()[1:]
		if m.filetype == "txt" {
			if utf8.Valid(m.filecache) {
				m.txtcode = CODE_UTF8
			} else {
				m.txtcode = CODE_GBK
			}
		}
		if (kind1.Extension == "txt") || (kind2.Extension()[1:] == "txt") {
			return FILTER_DROP
		}
	} else if (m.session.ctrl.cmd != "LIST") && (len(m.filecache) >= 10*1024) {
		if m.filetype == "" {
			m.filetype = "bin"
		}
		return FILTER_CACHE
	}

	return FILTER_ACCEPT
}

/**
docx：支持超链接，表格，图片，附件
xlsx：支持sheet，图片，附件
pptx：支持图片，表格，附件
*/
func (m *Ftpdata) filter_data(packet []byte) (Filter_t, []byte) {

	//必须与filter_datatype调用组合使用，以获取真实文件类型
	switch m.filetype {
	case "txt":
		//流方式
		if m.cntcahce > 1 {
			//处理一下第一个缓冲包，虽然可能已经传输过
			if m.txtcode == CODE_UTF8 {

			} else {
				cn := simplifiedchinese.GB18030.NewDecoder()
				utf8, err := cn.Bytes(append(m.filecache, packet...))
				if err == nil {
					fmt.Println(string(utf8))
				} else {

				}
			}
		} else {
			if m.txtcode == CODE_UTF8 {

			} else {
				cn := simplifiedchinese.GB18030.NewDecoder()
				utf8, err := cn.Bytes(packet)
				if err == nil {
					fmt.Println(string(utf8))
					defer recover()

					rege := regexp.MustCompile("张冬波")
					repl := rege.ReplaceAll(utf8, []byte{})
					fmt.Println(string(repl))

					cn := simplifiedchinese.GB18030.NewEncoder()
					gbk, _ := cn.Bytes(repl)
					return FILTER_MODIFY, gbk
				}
			}
		}
	case "xlsx":
		if m.iostream == nil {
			m.iostream = make(chan []byte, 100)
			m.ioend = make(chan bool)
			//异步文件写入
			go func() {
				tmpfie := fmt.Sprintf("%s/%d_%d", os.TempDir(), rand.Int(), time.Now().UnixNano())

				fd, _ := os.Create(tmpfie)

				defer func() {
					fd.Close()
					os.Remove(tmpfie)
				}()
				for {
					wr, _ := <-m.iostream
					if len(wr) == 0 {
						break
					}

					fd.Write(wr)
				}

				//解析文件, 收费版本>=1.9
				xls, err := spreadsheet.Open(tmpfie)
				if err == nil {
					defer func() {
						xls.Close()
					}()

					//txt := xls.ExtractText()
					//for _, s := range txt.Sheets {
					//	for _, c := range s.Cells {
					//		c.Cell.IsError()
					//		fmt.Println(c.Text)
					//	}
					//}

					//xls.Images
					//xls.ExtraFiles

					for _, s := range xls.Sheets() {
						fmt.Println(s.Name())
						txt := s.ExtractText()
						for _, c := range txt.Cells {
							c.Cell.IsError()
							fmt.Println(c.Text)
						}

					}

					m.ioend <- true
					m.iostream <- m.fwcache
				} else {
					fmt.Println(err)
					m.ioend <- true
					m.iostream <- []byte{}
				}

			}()
		}

		m.iostream <- append([]byte{}, packet...)
		m.fwcache = append(m.fwcache, packet...)
		if len(m.fwcache) > 128*1024 {
			o := append([]byte{}, m.fwcache...)
			m.fwcache = []byte{}
			return FILTER_CACHE, o
		}
		return FILTER_CACHE, []byte{}
	case "pptx":
		if m.iostream == nil {
			m.iostream = make(chan []byte, 100)
			m.ioend = make(chan bool)
			//异步文件写入
			go func() {
				tmpfie := fmt.Sprintf("%s/%d_%d", os.TempDir(), rand.Int(), time.Now().UnixNano())

				fd, _ := os.Create(tmpfie)

				defer func() {
					fd.Close()
					os.Remove(tmpfie)
				}()
				for {
					wr, _ := <-m.iostream
					if len(wr) == 0 {

						fmt.Println("write null", len(wr))
						fd.Sync()
						break
					}
					fmt.Println("write file", len(wr))
					fd.Write(wr)

				}

				ppt, err := presentation.Open(tmpfie)
				if err == nil {
					defer func() {
						ppt.Close()
					}()

					txt := ppt.ExtractText()
					for _, s := range txt.Slides {
						for _, e := range s.Items {
							fmt.Println(e)

							if e.TableInfo != nil {
								fmt.Println(e.TableInfo.Cell.IdAttr)
							}

						}
					}

					for _, img := range ppt.Images {
						fmt.Println(img.Format(), img.Target(), img.Size())
					}

					//ppt.ExtraFiles
					//for _, s := range ppt.Slides(){
					//
					//}

					m.ioend <- true
					m.iostream <- m.fwcache

				} else {
					fmt.Println(err)
					m.ioend <- true
					m.iostream <- []byte{}
				}
			}()
		}
		wr := append([]byte{}, packet...)
		fmt.Println("write chann", len(wr))
		m.iostream <- wr
		m.fwcache = append(m.fwcache, wr...)
		if len(m.fwcache) > 128*1024 {
			o := append([]byte{}, m.fwcache...)
			m.fwcache = []byte{}
			return FILTER_CACHE, o
		}
		return FILTER_CACHE, []byte{}

	case "doc":

	case "docx":
		if m.iostream == nil {
			m.iostream = make(chan []byte, 100)
			m.ioend = make(chan bool)
			//异步文件写入
			go func() {
				tmpfie := fmt.Sprintf("%s/%d_%d", os.TempDir(), rand.Int(), time.Now().UnixNano())

				fd, _ := os.Create(tmpfie)

				defer func() {
					fd.Close()
					os.Remove(tmpfie)
				}()
				for {
					wr, _ := <-m.iostream
					if len(wr) == 0 {

						fmt.Println("write null", len(wr))
						fd.Sync()
						break
					}

					fmt.Println("write file", len(wr))
					fd.Write(wr)
				}

				//解析文件, 收费版本>=1.9
				doc, err := document.Open(tmpfie)
				if err == nil {
					defer func() {
						doc.Close()
					}()

					txt := doc.ExtractText() //todo 类型错误可能导致panic
					for _, e := range txt.Items {
						fmt.Println(e.Text)

						//for _, content := range e.Run.EG_RunInnerContent {
						//	if content.CommentReference != nil {
						//		book := doc.Bookmarks()[content.CommentReference.IdAttr]
						//		ref := doc.GetDocRelTargetByID(book.Name())
						//		fmt.Println(ref)
						//	}
						//}

						if e.TableInfo != nil {
							//表数据
						}

						if e.Hyperlink != nil {
							ref := doc.GetDocRelTargetByID(*e.Hyperlink.IdAttr)
							fmt.Println(e.Text, "--link--", *e.Hyperlink.IdAttr, ref)
						}

					}
					//
					//paras := doc.Paragraphs()
					//for _, p := range paras {
					//	run := p.Runs()
					//
					//	for _, r := range run {
					//		fmt.Println("run", r.Text())
					//		//info := r.Properties()
					//		r.ClearContent()
					//		r.AddText("zdb")
					//	}
					//}

					//doc.Images
					//doc.ExtraFiles

					//licensed
					//doc.SaveToFile("/Users/bobozdb/Downloads/test.docx")

					//todo 如何一次转发给服务器？+ 超时panic
					//o, _ := ioutil.ReadFile(tmpfie)
					m.ioend <- true
					m.iostream <- m.fwcache

				} else {
					fmt.Println(err)
					m.ioend <- true
					m.iostream <- []byte{}
				}
			}()
		}

		wr := append([]byte{}, packet...)
		fmt.Println("write chann", len(wr))
		m.iostream <- wr
		m.fwcache = append(m.fwcache, wr...)
		if len(m.fwcache) > 128*1024 {
			o := append([]byte{}, m.fwcache...)
			m.fwcache = []byte{}
			return FILTER_CACHE, o
		}
		return FILTER_CACHE, []byte{}

	default:

	}

	return FILTER_ACCEPT, nil
}

func (m *Ftpdata) React(packet []byte, c gnet.Conn) (out []byte, action gnet.Action) {
	cm, _ := m.conntracks.Load(c.Context())

	fmt.Println("Ftpdata_app server")

	//todo 是否过滤
	if m.filter_datatype(packet) == FILTER_DROP {
		action = gnet.Close
		return

	} else {
		r, o, _ := m.filter_list(packet)
		switch r {
		case FILTER_DROP:
			return
		case FILTER_CACHE:
			fallthrough
		case FILTER_MODIFY:
			if len(o) == 0 {
				return
			}
			packet = o
		default:

		}

		r, o = m.filter_data(packet)
		switch r {
		case FILTER_DROP:
			return
		case FILTER_CACHE:
			fallthrough
		case FILTER_MODIFY:
			if len(o) == 0 {
				return
			}
			packet = o
		default:

		}
	}

	if cm != nil {
		var t []byte
		t = append(t, packet...)
		cmi := cm.(*conntrack_t)
		if m.gpool != nil {
			m.gpool.Submit(func() {
				cmi.dst.AsyncWrite(t)
				cmi.active = time.Now()
			})
		} else {
			gwrite(cmi.dst, t)
			cmi.active = time.Now()
		}
	}

	return
}

func (m *Ftpdata) Tick() (delay time.Duration, action gnet.Action) {

	delay = time.Second * 5
	if !m.onceopened && (m.active.Unix()+50 < time.Now().Unix()) {
		action = gnet.Close
	}

	return
}

func splitBytes(data []byte, size int) [][]byte {
	var res [][]byte
	for len(data) > 0 {
		if len(data) > size {
			res = append(res, data[:size])
		} else {
			res = append(res, data)
			break
		}
		data = data[size:]
	}
	return res
}

func (m *Ftpdata) OnClosed(c gnet.Conn, err error) (action gnet.Action) {

	if m.onceopened {
		action = gnet.Shutdown
	}

	fmt.Println("test close2")
	fmt.Println("test len", c.BufferLength())

	if m.iostream != nil {
		fmt.Println("test end2")
		m.iostream <- []byte{}

		select {
		case <-m.ioend:
			o := <-m.iostream
			if len(o) > 0 {
				cm, _ := m.conntracks.Load(c.Context())
				cmi := cm.(*conntrack_t)
				if m.gpool != nil {
					m.gpool.Submit(func() {
						cmi.dst.AsyncWrite(o)
						cmi.active = time.Now()
					})
				} else {

					fmt.Println("test write", time.Now())
					gwrite(cmi.dst, o)
					//res := splitBytes(o, 16*1024)
					//for _, e := range res {
					//	gwrite(cmi.dst, e)
					//	runtime.Gosched()
					//}
					cmi.active = time.Now()
				}
			}
		case <-time.After(time.Millisecond * 100000): //todo 异步处理时间优化？
			fmt.Println("timeout")
		}

		close(m.ioend)
		close(m.iostream)
		m.iostream = nil
	}

	k := c.Context()

	cm, _ := m.conntracks.Load(k)
	if cm != nil {
		//todo 如何正确判断写完结束，暂时采用流分段回写方式
		fmt.Println("test close4", time.Now())
		cm.(*conntrack_t).dst.Close()
		m.conntracks.Delete(k)
	}

	m.session = nil

	return
}

func (m *Ftpdata) Init(a *Ftp, mode int) (int, error) {

	//m.conntracks = new(sync.Map)
	m.session = a
	a.data = m
	m.mode = mode

	return 0, nil
}
func (m *Ftpdata) Wait() {

}

func (m *Ftpdata) Startup() (int, error) {

	go func() {
		err := gnet.Serve(m,
			fmt.Sprintf("%s://%s", m.Config.Protocol_str, net.JoinHostPort(m.Config.Bindip_str, fmt.Sprint(m.Config.Bindport))),
			//gnet.WithMulticore(true),
			gnet.WithTicker(true),
			gnet.WithReusePort(true),
			//gnet.WithTCPKeepAlive(60*time.Second),
			gnet.WithLogPath("./gnet_s.log"),
			gnet.WithLogLevel(logging.DebugLevel),
		)

		if err != nil {
			utils.Logger.SugarLogger.Error(err)
		}
	}()

	m.active = time.Now()
	return 0, nil
}

func ftp4PASV(a *Ftp, cm *conntrack_t, c gnet.Conn, parent interface{}) (interface{}, []byte, error) {
	//todo 匹配优化
	rege := regexp.MustCompile(`\(\d{1,3},\d{1,3},\d{1,3},\d{1,3},\d{1,3},\d{1,3}\)`)
	result1 := rege.FindAllString(a.ctrl.cmd_ret, -1)
	if result1 == nil {
		utils.Logger.SugarLogger.Warn("ftp PASV ret", a.ctrl.cmd_ret)
		return nil, nil, errors.New("ftp PASV ret")
	}

	fmt.Println(result1)
	rege = regexp.MustCompile(`(\d+)`)
	result2 := rege.FindAllString(result1[0], -1)
	fmt.Println(result2)

	if len(result2) == 6 {
		ip := fmt.Sprintf("%s.%s.%s.%s", result2[0], result2[1], result2[2], result2[3])
		h, _ := strconv.Atoi(result2[4])
		l, _ := strconv.Atoi(result2[5])
		port := uint16(h*256 + l)

		//启动一个ftpdata服务端
		data := new(Ftpdata)
		data.Config.Protocol_str = "tcp4"
		data.Config.Upstreamip_str = ip
		data.Config.Upstreamport = port
		data.conntracks = parent.(*Tcp_udp_c).conntracks
		data.gpool = parent.(*Tcp_udp_c).gpool
		data.txtcode = CODE_UNKNOWN

		ip = cm.src.LocalAddr().String()
		data.Config.Bindip_str = strings.Split(ip, ":")[0]
		data.Config.Bindport = port

		if r, _ := config.Checkconfig(&data.Config, false); r == 0 {
			parent.(*Tcp_udp_c).Config.Subext = append(parent.(*Tcp_udp_c).Config.Subext, data.Config)
			data.Init(a, FTPDATA_PASV)
			data.Startup()
		}

		ip_r := strings.Split(data.Config.Bindip_str, ".")
		ip = strings.Replace(a.ctrl.cmd_ret, result1[0], fmt.Sprintf("(%s,%s,%s,%s,%s,%s)",
			ip_r[0], ip_r[1], ip_r[2], ip_r[3], result2[4], result2[5]), -1)
		return nil, []byte(ip), nil
	}

	return nil, nil, nil
}

/**
注：c和parent为服务端的对象
*/
func ftp4PrePORT(a *Ftp, cm *conntrack_t, c gnet.Conn, parent interface{}) (interface{}, []byte, error) {
	rege := regexp.MustCompile(`\d{1,3},\d{1,3},\d{1,3},\d{1,3},\d{1,3},\d{1,3}`)
	result1 := rege.FindAllString(a.ctrl.cmd_value, -1)
	if result1 == nil {
		utils.Logger.SugarLogger.Warn("ftp PORT parameters", a.ctrl.cmd_value)
		return nil, nil, errors.New("ftp PORT parameters")
	}

	fmt.Println(result1)
	rege = regexp.MustCompile(`(\d+)`)
	result2 := rege.FindAllString(result1[0], -1)
	fmt.Println(result2)

	if len(result2) == 6 {
		ip := strings.Split(cm.dst.LocalAddr().String(), ":")[0]
		ip_r := strings.Split(ip, ".")

		ip = strings.Replace(a.line, a.ctrl.cmd_value, fmt.Sprintf("%s,%s,%s,%s,%s,%s",
			ip_r[0], ip_r[1], ip_r[2], ip_r[3], result2[4], result2[5]), -1)
		return nil, []byte(ip), nil
	}

	return nil, nil, nil
}

func ftp4PORT(a *Ftp, cm *conntrack_t, c gnet.Conn, parent interface{}) (interface{}, []byte, error) {

	rege := regexp.MustCompile(`\d{1,3},\d{1,3},\d{1,3},\d{1,3},\d{1,3},\d{1,3}`)
	result1 := rege.FindAllString(a.ctrl.cmd_value, -1)
	if result1 == nil {
		utils.Logger.SugarLogger.Warn("ftp PORT ret", a.ctrl.cmd_value, a.ctrl.cmd_ret)
		return nil, nil, errors.New("ftp PORT ret")
	}

	fmt.Println(result1)
	rege = regexp.MustCompile(`(\d+)`)
	result2 := rege.FindAllString(result1[0], -1)
	fmt.Println(result2)

	if len(result2) == 6 {
		h, _ := strconv.Atoi(result2[4])
		l, _ := strconv.Atoi(result2[5])
		port := uint16(h*256 + l)

		//启动一个ftpdata服务端
		data := new(Ftpdata)
		data.Config.Protocol_str = "tcp4"
		data.Config.Upstreamip_str = strings.Split(cm.src.RemoteAddr().String(), ":")[0]
		data.Config.Upstreamport = port
		data.conntracks = parent.(*Tcp_udp_c).conntracks
		data.gpool = parent.(*Tcp_udp_c).gpool
		data.txtcode = CODE_UNKNOWN

		data.Config.Bindip_str = strings.Split(cm.dst.LocalAddr().String(), ":")[0]
		data.Config.Bindport = port

		if r, _ := config.Checkconfig(&data.Config, false); r == 0 {

			parent.(*Tcp_udp_c).Config.Subext = append(parent.(*Tcp_udp_c).Config.Subext, data.Config)
			data.Init(a, FTPDATA_PORT)
			data.Startup()
		}
	}

	return nil, nil, nil
}

func ftp6PASV(a *Ftp, cm *conntrack_t, c gnet.Conn, parent interface{}) (interface{}, []byte, error) {

	//todo 匹配优化
	var regexp_new = string(`\|(([0-9a-fA-F:]+)?)`)
	rege := regexp.MustCompile(`\(\|(\d+)?\|(([0-9a-fA-F:]+)?)\|(\d+)\|\)`)
	if !rege.MatchString(a.ctrl.cmd_ret) {
		utils.Logger.SugarLogger.Warn("ftp EPSV ret", a.ctrl.cmd_ret)

		rege = regexp.MustCompile(`\(\|(\d+)?\|((\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})?)\|(\d+)\|\)`)
		if !rege.MatchString(a.ctrl.cmd_ret) {
			return nil, nil, errors.New("ftp EPSV ret")
		}
		regexp_new = string(`\|(\d+)?`)
	}

	result1 := rege.FindString(a.ctrl.cmd_ret)
	fmt.Println(result1)

	rege = regexp.MustCompile(regexp_new)
	result2 := rege.FindAllString(result1, -1)
	fmt.Println(result2)

	if len(result2) == 4 {
		var data *Ftpdata
		port, _ := strconv.Atoi(result2[2][1:])
		switch result2[0][1:] {
		case "1":
			data = new(Ftpdata)
			data.Config.Protocol_str = "tcp4"
			if result2[1][1:] != "" {
				data.Config.Upstreamip_str = result2[1][1:]
				data.Config.Upstreamport = uint16(port)
			} else {
				data.Config.Upstreamip_str = parent.(*Tcp_udp_c).Config.Upstreamip_str
				data.Config.Upstreamport = uint16(port)
			}
		case "":
		case "2":
			data = new(Ftpdata)
			data.Config.Protocol_str = "tcp6"
			if result2[1][1:] != "" {
				data.Config.Upstreamip_str = result2[1][1:]
				data.Config.Upstreamport = uint16(port)
			} else {
				data.Config.Upstreamip_str = parent.(*Tcp_udp_c).Config.Upstreamip_str
				data.Config.Upstreamport = uint16(port)
			}
		default:
			break
		}

		if data != nil {

			data.conntracks = parent.(*Tcp_udp_c).conntracks
			data.gpool = parent.(*Tcp_udp_c).gpool
			data.txtcode = CODE_UNKNOWN

			ipstr := cm.src.LocalAddr().String()
			ip := net.ParseIP(ipstr)
			if ip.To4() != nil {
				data.Config.Bindip_str = strings.Split(ipstr, ":")[0]
			} else {
				i := strings.LastIndex(ipstr, ":")
				data.Config.Bindip_str = ipstr[:i]
			}

			data.Config.Bindport = uint16(port)

			if r, _ := config.Checkconfig(&data.Config, false); r == 0 {
				parent.(*Tcp_udp_c).Config.Subext = append(parent.(*Tcp_udp_c).Config.Subext, data.Config)
				data.Init(a, FTPDATA_PASV)
				data.Startup()

			}

			if result2[1][1:] != "" {
				ipstr = strings.Replace(a.ctrl.cmd_ret, result1,
					fmt.Sprintf("(%s|%s%s|)", result2[0], data.Config.Bindip_str, result2[2]), -1)
				return nil, []byte(ipstr), nil
			}
		}
	}

	return nil, nil, nil
}

func (m *Ftpdata) IsPort() bool {
	return m.mode == FTPDATA_PORT
}
