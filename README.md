# gope

总体目标是构建纯Golang语言实现的网络代理与交换服务端，同时作为学习Go语言的基础

gope(Go Proxy Expert)目前还属于demo阶段，版本定义为v0.0.1-alpha

参考学习: https://github.com/snail007/goproxy

## _2024-03_
* 1. 基于gnet1.6实现支持TCP和UDP代理
* 2. 支持FTP协议，详情查看./app/ftp.go；过滤功能暂且只是演示实现一些特性的扩展使用方法


### _cli使用说明_
```
1. ./gope 默认启动tcp://127.0.0.1的代理，目的端口是本地54321到12345
2. ./gope -t 54321 -T 21 -I 'real server ip' -a ftp 举例是启动ftp代理

```

## _2024-04_
* 1. 支持http和https，https借用了net/http的ReverseProxy框架实现（方便tls的透明处理）；过滤功能重点是对xss的新特性试验
