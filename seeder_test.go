package main

import (
	"github.com/btcsuite/btcd/wire"
	"net"
	"strconv"
	"testing"
)

func TestGetNonStdIP(t *testing.T) {

	var iptests = []struct {
		rip   string
		port  uint16
		encip string
	}{
		{"1.2.3.4", 1234, "137.195.4.210"},
		{"50.123.45.67", 43210, "101.165.168.202"},
		{"202.36.170.3", 65535, "199.31.255.255"},
		{"123.213.132.231", 34, "12.91.0.34"},
	}

	for _, atest := range iptests {
		newip := getNonStdIP(net.ParseIP(atest.rip), atest.port)
		if newip.String() != atest.encip {
			t.Errorf("real-ip: %s real-port: %v encoded-ip: %v expected-ip: %s", atest.rip, atest.port, newip, atest.encip)
		}
	}
}

func TestAddnNa(t *testing.T) {
	// create test data struct
	var td = []struct {
		ip      string
		port    int
		dnsType uint32
	}{
		{"1.2.3.4", 28333, 1},
		{"50.123.45.67", 43210, 2},
	}

	s := &dnsseeder{
		port:    28333,
		pver:    1234,
		maxSize: 1,
	}
	s.theList = make(map[string]*node)

	for _, atest := range td {
		// Test NewNetAddress.
		tcpAddr := &net.TCPAddr{
			IP:   net.ParseIP(atest.ip),
			Port: atest.port,
		}
		na, _ := wire.NewNetAddress(tcpAddr, 0)
		ndName := net.JoinHostPort(na.IP.String(), strconv.Itoa(int(na.Port)))

		result := s.addNa(na)
		if result != true {
			t.Errorf("failed to create new node: %s", ndName)
		}
		if s.theList[ndName].dnsType != atest.dnsType {
			t.Errorf("node: %s dnsType:%v expected: %v", ndName, s.theList[ndName].dnsType, atest.dnsType)
		}
	}

	tcpAddr := &net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 1234,
	}
	na, _ := wire.NewNetAddress(tcpAddr, 0)
	result := s.addNa(na)

	if result != false {
		t.Errorf("node added but should have failed as seeder full: %s", net.JoinHostPort(na.IP.String(), strconv.Itoa(int(na.Port))))
	}

	tcpAddr = &net.TCPAddr{
		IP:   net.ParseIP("1.2.3.4"),
		Port: 28333,
	}
	na, _ = wire.NewNetAddress(tcpAddr, 0)
	result = s.addNa(na)

	if result != false {
		t.Errorf("node added but should have failed as duplicate: %s", net.JoinHostPort(na.IP.String(), strconv.Itoa(int(na.Port))))
	}

}

/*

 */
