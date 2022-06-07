package main

import (
	"fmt"
	"io"
	"net"
)

type TcpForward struct {
	SrcAddr  string
	DestAddr string
}

func NewTcpForward(src, dest string) *TcpForward {
	return &TcpForward{src, dest}
}

func (t *TcpForward) StartForward() {
	//启动TCP监听
	listen, err := net.Listen("tcp", t.SrcAddr)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer listen.Close()
	for {
		//不断的返回监听到的连接
		conn, err := listen.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("TCP:Accept 一个连接")
		//处理监听到的连接,这里就是需要把流量转发到目标地址
		go handleRequest(conn, t.DestAddr)
	}
}

//将conn的数据转发到目标地址,并且从目标地址获取数据返回到conn
func handleRequest(conn net.Conn, dest string) {
	//连接目标地址
	forwardConn, err := net.Dial("tcp", dest)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("TCP:连接到远程,开始转发数据")
	go copyIO(conn, forwardConn)
	go copyIO(forwardConn, conn)
}

func copyIO(src, dest net.Conn) {
	defer src.Close()
	defer dest.Close()
	io.Copy(src, dest)
}
