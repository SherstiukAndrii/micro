package utils

import "net"

const ConfigServerPortPath = "/var/tmp/config_server_port.txt"

func GetFreePort() int {
	for {
		l, err := net.Listen("tcp", ":0")
		if err != nil {
			continue
		}
		port := l.Addr().(*net.TCPAddr).Port
		l.Close()
		return port
	}
}
