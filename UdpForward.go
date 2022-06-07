package main

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

const bufferSize = 40960
const DefaultTimeout = time.Minute * 5

type udpForward struct {
	src          *net.UDPAddr
	dest         *net.UDPAddr
	client       *net.UDPAddr
	listenerConn *net.UDPConn

	connections      map[string]*connection
	connectionsMutex *sync.RWMutex

	connectCallback    func(addr string)
	disconnectCallback func(addr string)

	timeout time.Duration

	closed bool
}

type connection struct {
	available  chan struct{}
	udp        *net.UDPConn
	lastActive time.Time
}

func NewUdpForward(src, dest string) *udpForward {
	u := new(udpForward)
	u.connectCallback = func(addr string) {}
	u.disconnectCallback = func(addr string) {}
	u.connectionsMutex = new(sync.RWMutex)
	u.connections = make(map[string]*connection)
	u.timeout = DefaultTimeout

	var err error
	//将string的ip类型转换成UDPAddr,本机的
	u.src, err = net.ResolveUDPAddr("udp", src)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	//将string的ip类型转换成UDPAddr,目标地址的
	u.dest, err = net.ResolveUDPAddr("udp", dest)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	//启动UDP监听
	u.listenerConn, err = net.ListenUDP("udp", u.src)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	fmt.Println("启动UDP监听" + src)
	return u
}

func (u *udpForward) StartUdpForward() {
	go u.janitor()
	for {
		buf := make([]byte, bufferSize)
		n, addr, err := u.listenerConn.ReadFromUDP(buf)
		if err != nil {
			log.Println("forward: failed to read, terminating:", err)
			return
		}
		fmt.Println("UDP:读取UDP的数据,待转发处理")
		go u.handle(buf[:n], addr)
	}
}

func (u *udpForward) janitor() {
	for !u.closed {
		time.Sleep(u.timeout)
		var keysToDelete []string

		u.connectionsMutex.RLock()
		for k, conn := range u.connections {
			if conn.lastActive.Before(time.Now().Add(-u.timeout)) {
				keysToDelete = append(keysToDelete, k)
			}
		}
		u.connectionsMutex.RUnlock()

		u.connectionsMutex.Lock()
		for _, k := range keysToDelete {
			u.connections[k].udp.Close()
			delete(u.connections, k)
		}
		u.connectionsMutex.Unlock()

		for _, k := range keysToDelete {
			u.disconnectCallback(k)
		}
	}
}

func (u *udpForward) handle(data []byte, addr *net.UDPAddr) {
	u.connectionsMutex.Lock()
	conn, found := u.connections[addr.String()]
	if !found {
		u.connections[addr.String()] = &connection{
			available:  make(chan struct{}),
			udp:        nil,
			lastActive: time.Now(),
		}
	}
	u.connectionsMutex.Unlock()

	if !found {
		var udpConn *net.UDPConn
		var err error
		if u.dest.IP.To4()[0] == 127 {
			// log.Println("using local listener")
			laddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:")
			udpConn, err = net.DialUDP("udp", laddr, u.dest)
		} else {
			udpConn, err = net.DialUDP("udp", nil, u.dest)
		}
		if err != nil {
			log.Println("udp-forward: failed to dial:", err)
			delete(u.connections, addr.String())
			return
		}

		u.connectionsMutex.Lock()
		u.connections[addr.String()].udp = udpConn
		u.connections[addr.String()].lastActive = time.Now()
		close(u.connections[addr.String()].available)
		u.connectionsMutex.Unlock()

		u.connectCallback(addr.String())

		_, _, err = udpConn.WriteMsgUDP(data, nil, nil)
		if err != nil {
			log.Println("udp-forward: error sending initial packet to client", err)
		}
		fmt.Println("UDP:处理读取UDP的数据,转发成功")
		for {
			// log.Println("in loop to read from NAT connection to servers")
			buf := make([]byte, bufferSize)
			oob := make([]byte, bufferSize)
			n, _, _, _, err := udpConn.ReadMsgUDP(buf, oob)
			if err != nil {
				u.connectionsMutex.Lock()
				udpConn.Close()
				delete(u.connections, addr.String())
				u.connectionsMutex.Unlock()
				u.disconnectCallback(addr.String())
				log.Println("udp-forward: abnormal read, closing:", err)
				return
			}

			// log.Println("sent packet to client")
			_, _, err = u.listenerConn.WriteMsgUDP(buf[:n], nil, addr)
			if err != nil {
				log.Println("udp-forward: error sending packet to client:", err)
			}
		}

		// unreachable
	}

	<-conn.available

	// log.Println("sent packet to server", conn.udp.RemoteAddr())
	_, _, err := conn.udp.WriteMsgUDP(data, nil, nil)
	if err != nil {
		log.Println("udp-forward: error sending packet to server:", err)
	}

	shouldChangeTime := false
	u.connectionsMutex.RLock()
	if _, found := u.connections[addr.String()]; found {
		if u.connections[addr.String()].lastActive.Before(
			time.Now().Add(u.timeout / 4)) {
			shouldChangeTime = true
		}
	}
	u.connectionsMutex.RUnlock()

	if shouldChangeTime {
		u.connectionsMutex.Lock()
		// Make sure it still exists
		if _, found := u.connections[addr.String()]; found {
			connWrapper := u.connections[addr.String()]
			connWrapper.lastActive = time.Now()
			u.connections[addr.String()] = connWrapper
		}
		u.connectionsMutex.Unlock()
	}
}
