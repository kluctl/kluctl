package port_tool

import (
	"fmt"
	"net"
	"sync"
)

var nextPort = 30000
var mutex sync.Mutex

func NextFreePort(ip string) int {
	mutex.Lock()
	defer mutex.Unlock()

	for {
		a := net.TCPAddr{
			IP:   net.ParseIP(ip),
			Port: nextPort,
		}
		var ra net.TCPAddr
		nextPort++
		c, err := net.DialTCP("tcp", &a, &ra)
		if err == nil {
			c.Close()
			continue
		}
		return a.Port
	}
}

func NewListenerWithUniquePort(host string) net.Listener {
	mutex.Lock()
	defer mutex.Unlock()

	for {
		l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, nextPort))
		nextPort++
		if err == nil {
			return l
		}
	}
}
