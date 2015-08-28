package main

import (
	"net"
	"time"

	"github.com/btcsuite/btcd/wire"
)

// Twistee struct contains details on one twister client
type Twistee struct {
	na           *wire.NetAddress
	lastConnect  time.Time
	lastTry      time.Time
	crawlStart   time.Time
	statusTime   time.Time
	crawlActive  bool
	connectFails uint32
	statusStr    string           // string with last error or OK details
	version      int32            // remote client protocol version
	strVersion   string           // remote client user agent
	services     wire.ServiceFlag // remote client supported services
	lastBlock    int32
	status       uint32 // rg,cg,wg,ng
	rating       uint32 // if it reaches 100 then we ban them
	nonstdIP     net.IP
	dnsType      uint32
}

// status2str will return the string description of the status
func (tw Twistee) status2str() string {
	switch tw.status {
	case statusRG:
		return "statusRG"
	case statusCG:
		return "statusCG"
	case statusWG:
		return "statusWG"
	case statusNG:
		return "statusNG"
	default:
		return "Unknown"
	}
}

// dns2str will return the string description of the dns type
func (tw Twistee) dns2str() string {
	switch tw.dnsType {
	case DNSV4STD:
		return "v4 standard port"
	case DNSV4NON:
		return "v4 non-standard port"
	case DNSV6STD:
		return "v6 standard port"
	case DNSV6NON:
		return "v6 non-standard port"
	default:
		return "Unknown DNS Type"
	}
}

/*

 */
