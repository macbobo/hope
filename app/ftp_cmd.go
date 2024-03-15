package app

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type Ftpcmd struct {
	cmd       string
	cmd_value string
	cmd_ret   string
}

var ftp_rfc = []string{
	//959
	"^USER",
	"^PASS",
	"^ACCT",
	"^LIST",
	"^NLST",
	"^PORT",
	"^PASV",
	"^QUIT",
	"^PWD",
	"^CWD",
	"^CDUP",
	"^TYPE [I,A]",
	"^SIZE",
	"^RETR",
	"^STOR",
	"^REST",
	"^ABOT",
	"^RNFR",
	"^RNTO",
	"^DELE",
	"^MKD",
	"^RMD",
	"^SYST",
	"^SITE",
	"^STAT",
	"^OPTS",
	"^MDTM",
	"^HELP",
	"^NOOP",
	"^SMNT",
	"^REIN",
	"^STRU",
	"^MODE",
	"^STOU",
	"^APPE",
	"^ALLO",

	//
	"^EPRT",
	"^EPSV",
}

func (c *Ftpcmd) Check(cmd string) int {
	cmd_u := strings.ToUpper(cmd)

	for _, v := range ftp_rfc {
		//if strings.HasPrefix(cmd_u, v) {
		//	c.cmd = v
		//	c.cmd_value = strings.TrimSpace(cmd[len(c.cmd):])
		//}

		if m, _ := regexp.Match(v, []byte(cmd_u)); m {
			c.cmd = v[1:]
			if strings.HasPrefix(c.cmd, "TYPE") {
				c.cmd = strings.Split(c.cmd, " ")[0]
			}
			c.cmd_value = strings.TrimSpace(cmd[len(c.cmd):])
		}
	}

	fmt.Println("ftp", c.cmd)

	return 0
}

func (c *Ftpcmd) Setret(ret string) (ok bool) {
	ok = false

	if len(c.cmd) > 0 {
		c.cmd_ret = ret
		switch c.cmd_ret[0] {
		case '1':
			fallthrough
		case '2':
			ok = true
		case '4':
		case '5':
		default:

		}

	}
	fmt.Println("ftp", c)
	return
}
func (c *Ftpcmd) blacklist() (Filter_t, error) {
	return FILTER_ACCEPT, nil
}

func (c *Ftpcmd) whitelist() (Filter_t, error) {
	return FILTER_ACCEPT, nil
}

func (c *Ftpcmd) Clear() {
	c.cmd = ""
	c.cmd_value = ""
	c.cmd_ret = ""
}

func (c *Ftpcmd) filter() (Filter_t, error) {

	switch c.cmd {
	case "STOR", "RETR", "DELE", "SIZE":
		//文件名处理
		fmt.Println("ftp file", c.cmd_value)
		cmd := strings.ToLower(c.cmd_value)
		//todo 中文后缀和正则
		if strings.HasSuffix(cmd, ".txt") {
			return FILER_RETJECT, errors.New("550 gope returns\r\n")
		}
	case "MKD", "RMD", "LIST", "NLST":
		//文件目录处理
		fmt.Println("ftp dir", c.cmd_value)

	default:

	}

	return FILTER_ACCEPT, nil
}
