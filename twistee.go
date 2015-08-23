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
