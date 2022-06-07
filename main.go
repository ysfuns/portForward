package main

import (
	"flag"
	"fmt"
	"os"
)

var localPort string       //监听的本地端口 0.0.0.0:13659
var portForwardAddr string //端口转发的目标地址

func main() {
	//默认监听本地13658端口
	flag.StringVar(&localPort, "localport", ":13658", "-localport=0.0.0.0:9897 指定服务监听的端口")
	//默认转发到本机的13659端口
	flag.StringVar(&portForwardAddr, "portForwardAddr", "127.0.0.1:13659", "-portForwardAddr=127.0.0.1:1789 指定后端的IP和端口")
	flag.Parse()

	if localPort == "" || portForwardAddr == "" {
		fmt.Println("参数不正确")
		os.Exit(1)
	}

	go tcpProxy()
	udpProxy()
}

func tcpProxy() {
	t := NewTcpForward(localPort, portForwardAddr)
	t.StartForward()
}

func udpProxy() {
	u := NewUdpForward(localPort, portForwardAddr)
	u.StartUdpForward()
}
