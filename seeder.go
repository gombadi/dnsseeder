package main

import (
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/btcsuite/btcd/wire"
)

const (

	// NOUNCE is used to check if we connect to ourselves
	// as we don't listen we can use a fixed value
	nounce  = 0x0539a019ca550825
	minPort = 0
	maxPort = 65535

	crawlDelay = 22 // seconds between start crawlwer ticks
	auditDelay = 22 // minutes between audit channel ticks
	dnsDelay   = 57 // seconds between updates to active dns record list

	maxFails = 58 // max number of connect fails before we delete a node. Just over 24 hours(checked every 33 minutes)

	maxTo = 250 // max seconds (4min 10 sec) for all comms to node to complete before we timeout
)

const (
	dnsInvalid  = iota //
	dnsV4Std           // ip v4 using network standard port
	dnsV4Non           // ip v4 using network non standard port
	dnsV6Std           // ipv6 using network standard port
	dnsV6Non           // ipv6 using network non standard port
	maxDNSTypes        // used in main to allocate slice
)

const (
	// node status
	statusRG       = iota // reported good status. A remote node has reported this ip but we have not connected
	statusCG              // confirmed good. We have connected to the node and received addresses
	statusWG              // was good. node was confirmed good but now having problems
	statusNG              // no good. Will be removed from theList after 24 hours to redure bouncing ip addresses
	maxStatusTypes        // used in main to allocate slice
)

type dnsseeder struct {
	id        wire.BitcoinNet  // Magic number - Unique ID for this network. Sent in header of all messages
	theList   map[string]*node // the list of current nodes
	mtx       sync.RWMutex     // protect thelist
	maxSize   int              // max number of clients before we start restricting new entries
	port      uint16           // default network port this seeder uses
	pver      uint32           // minimum block height for the seeder
	ttl       uint32           // DNS TTL to use for this seeder
	dnsHost   string           // dns host we will serve results for this domain
	name      string           // Short name for the network
	desc      string           // Long description for the network
	initialIP string           // Initial ip address to connect to and ask for addresses if we have no seeders
	seeders   []string         // slice of seeders to pull ip addresses when starting this seeder
	maxStart  []uint32         // max number of goroutines to start each run for each status type
	delay     []int64          // number of seconds to wait before we connect to a known client for each status
	counts    NodeCounts       // structure to hold stats for this seeder
}

// initCrawlers needs to be run before the startCrawlers so it can get
// a list of current ip addresses from the other seeders and therefore
// start the crawl process
func (s *dnsseeder) initCrawlers() {

	for _, aseeder := range s.seeders {
		c := 0

		if aseeder == "" {
			continue
		}
		newRRs, err := net.LookupHost(aseeder)
		if err != nil {
			log.Printf("%s: unable to do initial lookup to seeder %s %v\n", s.name, aseeder, err)
			continue
		}

		for _, ip := range newRRs {
			if newIP := net.ParseIP(ip); newIP != nil {
				// 1 at the end is the services flag
				if x := s.addNa(wire.NewNetAddressIPPort(newIP, s.port, 1)); x == true {
					c++
				}
			}
		}
		if config.verbose {
			log.Printf("%s: completed import of %v addresses from %s\n", s.name, c, aseeder)
		}
	}

	// load one ip address into system and start crawling from it
	if len(s.theList) == 0 && s.initialIP != "" {
		if newIP := net.ParseIP(s.initialIP); newIP != nil {
			// 1 at the end is the services flag
			if x := s.addNa(wire.NewNetAddressIPPort(newIP, s.port, 1)); x == true {
				log.Printf("%s: crawling with initial IP %s \n", s.name, s.initialIP)
			}
		}
	}

	if len(s.theList) == 0 {
		log.Printf("%s: Error: No ip addresses from seeders so I have nothing to crawl.\n", s.name)
		for _, v := range s.seeders {
			log.Printf("%s: Seeder: %s\n", s.name, v)
		}
		log.Printf("%s: Initial IP: %s\n", s.name, s.initialIP)
	}
}

// startCrawlers is called on a time basis to start maxcrawlers new
// goroutines if there are spare goroutine slots available
func (s *dnsseeder) startCrawlers() {

	tcount := len(s.theList)
	if tcount == 0 {
		if config.debug {
			log.Printf("%s - debug - startCrawlers fail: no node ailable\n", s.name)
		}
		return
	}

	// struct to hold config options for each status
	var crawlers = []struct {
		desc       string
		status     uint32
		maxCount   uint32 // max goroutines to start for this status type
		totalCount uint32 // stats count of this type
		started    uint32 // count of goroutines started for this type
		delay      int64  // number of second since last try
	}{
		{"statusRG", statusRG, s.maxStart[statusRG], 0, 0, s.delay[statusRG]},
		{"statusCG", statusCG, s.maxStart[statusCG], 0, 0, s.delay[statusCG]},
		{"statusWG", statusWG, s.maxStart[statusWG], 0, 0, s.delay[statusWG]},
		{"statusNG", statusNG, s.maxStart[statusNG], 0, 0, s.delay[statusNG]},
	}

	s.mtx.RLock()
	defer s.mtx.RUnlock()

	// step through each of the status types RG, CG, WG, NG
	for _, c := range crawlers {

		// range on a map will not return items in the same order each time
		// so this is a random'ish selection
		for _, nd := range s.theList {

			if nd.status != c.status {
				continue
			}

			// stats count
			c.totalCount++

			if nd.crawlActive == true {
				continue
			}

			if c.started >= c.maxCount {
				continue
			}

			if (time.Now().Unix() - c.delay) <= nd.lastTry.Unix() {
				continue
			}

			// all looks good so start a go routine to crawl the remote node
			go crawlNode(s, nd)
			c.started++
		}

		if config.stats {
			log.Printf("%s: started crawler: %s total: %v started: %v\n", s.name, c.desc, c.totalCount, c.started)
		}

		// update the global stats in another goroutine to free the main goroutine
		// for other work
		go updateNodeCounts(s, c.status, c.totalCount, c.started)
	}

	if config.stats {
		log.Printf("%s: crawlers started. total clients: %d\n", s.name, tcount)
	}

	// returns and read lock released
}

// isDup will return true or false depending if the ip exists in theList
func (s *dnsseeder) isDup(ipport string) bool {
	s.mtx.RLock()
	_, dup := s.theList[ipport]
	s.mtx.RUnlock()
	return dup
}

// isNaDup returns true if this wire.NetAddress is already known to us
func (s *dnsseeder) isNaDup(na *wire.NetAddress) bool {
	return s.isDup(net.JoinHostPort(na.IP.String(), strconv.Itoa(int(na.Port))))
}

// addNa validates and adds a network address to theList
func (s *dnsseeder) addNa(nNa *wire.NetAddress) bool {

	// as this is run in many different goroutines then they may all try and
	// add new addresses so do a final check
	if s.isFull() {
		return false
	}

	if dup := s.isNaDup(nNa); dup == true {
		return false
	}
	if nNa.Port <= minPort || nNa.Port >= maxPort {
		return false
	}

	// if the reported timestamp suggests the netaddress has not been seen in the last 24 hours
	// then ignore this netaddress
	if (time.Now().Add(-(time.Hour * 24))).After(nNa.Timestamp) {
		return false
	}

	nt := node{
		na:          nNa,
		lastConnect: time.Now(),
		version:     0,
		status:      statusRG,
		statusTime:  time.Now(),
		dnsType:     dnsV4Std,
	}

	// select the dns type based on the remote address type and port
	if x := nt.na.IP.To4(); x == nil {
		// not ipv4
		if nNa.Port != s.port {
			nt.dnsType = dnsV6Non

			// produce the nonstdIP
			nt.nonstdIP = getNonStdIP(nt.na.IP, nt.na.Port)

		} else {
			nt.dnsType = dnsV6Std
		}
	} else {
		// ipv4
		if nNa.Port != s.port {
			nt.dnsType = dnsV4Non

			// force ipv4 address into a 4 byte buffer
			nt.na.IP = nt.na.IP.To4()

			// produce the nonstdIP
			nt.nonstdIP = getNonStdIP(nt.na.IP, nt.na.Port)
		}
	}

	// generate the key and add to theList
	k := net.JoinHostPort(nNa.IP.String(), strconv.Itoa(int(nNa.Port)))
	s.mtx.Lock()
	// final check to make sure another crawl & goroutine has not already added this client
	if _, dup := s.theList[k]; dup == false {
		s.theList[k] = &nt
	}
	s.mtx.Unlock()

	return true
}

// getNonStdIP is given an IP address and a port and returns a fake IP address
// that is encoded with the original IP and port number. Remote clients can match
// the two and work out the real IP and port from the two IP addresses.
func getNonStdIP(rip net.IP, port uint16) net.IP {

	b := []byte{0x0, 0x0, 0x0, 0x0}
	crcAddr := crc16(rip.To4())
	b[0] = byte(crcAddr >> 8)
	b[1] = byte((crcAddr & 0xff))
	b[2] = byte(port >> 8)
	b[3] = byte(port & 0xff)

	encip := net.IPv4(b[0], b[1], b[2], b[3])
	if config.debug {
		log.Printf("debug - encode nonstd - realip: %s port: %v encip: %s crc: %x\n", rip.String(), port, encip.String(), crcAddr)
	}

	return encip
}

// crc16 produces a crc16 from a byte slice
func crc16(bs []byte) uint16 {
	var x, crc uint16
	crc = 0xffff

	for _, v := range bs {
		x = crc>>8 ^ uint16(v)
		x ^= x >> 4
		crc = (crc << 8) ^ (x << 12) ^ (x << 5) ^ x
	}
	return crc
}

func (s *dnsseeder) auditNodes() {

	c := 0

	// set this early so for this audit run all NG clients will be purged
	// and space will be made for new, possible CG clients
	iAmFull := s.isFull()

	// cgGoal is 75% of the max statusCG clients we can crawl with the current network delay & maxStart settings.
	// This allows us to cycle statusCG users to keep the list fresh
	cgGoal := int(float64(float64(s.delay[statusCG]/crawlDelay)*float64(s.maxStart[statusCG])) * 0.75)
	cgCount := 0

	log.Printf("%s: Audit start. statusCG Goal: %v System Uptime: %s\n", s.name, cgGoal, time.Since(config.uptime).String())

	s.mtx.Lock()
	defer s.mtx.Unlock()

	for k, nd := range s.theList {

		if nd.crawlActive == true {
			if time.Now().Unix()-nd.crawlStart.Unix() >= 300 {
				log.Printf("warning - long running crawl > 5 minutes ====\n- %s status:rating:fails %v:%v:%v crawl start: %s last status: %s\n====\n",
					k,
					nd.status,
					nd.rating,
					nd.connectFails,
					nd.crawlStart.String(),
					nd.statusStr)
			}
		}

		// Audit task is to remove node that we have not been able to connect to
		if nd.status == statusNG && nd.connectFails > maxFails {
			if config.verbose {
				log.Printf("%s: purging node %s after %v failed connections\n", s.name, k, nd.connectFails)
			}

			c++
			// remove the map entry and mark the old node as
			// nil so garbage collector will remove it
			s.theList[k] = nil
			delete(s.theList, k)
		}

		// If seeder is full then remove old NG clients and fill up with possible new CG clients
		if nd.status == statusNG && iAmFull {
			if config.verbose {
				log.Printf("%s: seeder full purging node %s\n", s.name, k)
			}

			c++
			// remove the map entry and mark the old node as
			// nil so garbage collector will remove it
			s.theList[k] = nil
			delete(s.theList, k)
		}

		// check if we need to purge statusCG to freshen the list
		if nd.status == statusCG {
			if cgCount++; cgCount > cgGoal {
				// we have enough statusCG clients so purge remaining to cycle through the list
				if config.verbose {
					log.Printf("%s: seeder cycle statusCG - purging node %s\n", s.name, k)
				}

				c++
				// remove the map entry and mark the old node as
				// nil so garbage collector will remove it
				s.theList[k] = nil
				delete(s.theList, k)
			}

		}

	}
	if config.verbose {
		log.Printf("%s: Audit complete. %v nodes purged\n", s.name, c)
	}

}

// teatload loads the dns records with time based test data
func (s *dnsseeder) loadDNS() {
	updateDNS(s)
}

// isFull returns true if the number of remote clients is more than we want to store
func (s *dnsseeder) isFull() bool {
	if len(s.theList) > s.maxSize {
		return true
	}
	return false
}

// getSeederByName returns a pointer to the seeder based on its name or nil if not found
func getSeederByName(name string) *dnsseeder {
	for _, s := range config.seeders {
		if s.name == name {
			return s
		}
	}
	return nil
}

/*

 */
