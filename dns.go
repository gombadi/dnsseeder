package main

import (
	"log"
	"sync"

	"github.com/miekg/dns"
)

type currentIPs struct {
	ipv4std []dns.RR
	ipv4non []dns.RR
	ipv6std []dns.RR
	ipv6non []dns.RR
	mtx     sync.RWMutex
}

// latest holds the slices of current ip addresses
var latest currentIPs

// getLatestaRR returns a pointer to the latest slice of current dns.RR type
// dns.A records to pass back to the remote client
func getv4stdRR() []dns.RR { return latest.ipv4std }
func getv4nonRR() []dns.RR { return latest.ipv4non }
func getv6stdRR() []dns.RR { return latest.ipv6std }
func getv6nonRR() []dns.RR { return latest.ipv6non }

// updateDNS updates the current slices of dns.RR so incoming requests get a
// fast answer
func updateDNS(s *Seeder) {

	var rr4std, rr4non, rr6std, rr6non []dns.RR

	s.mtx.RLock()

	// loop over each dns recprd type we need
	for t := range []int{DNSV4STD, DNSV4NON, DNSV6STD, DNSV6NON} {

		numRR := 0

		for _, tw := range s.theList {
			// when we reach max exit
			if numRR >= 25 {
				break
			}

			if tw.status != statusCG {
				continue
			}

			if t == DNSV4STD || t == DNSV4NON {
				if t == DNSV4STD && tw.dnsType == DNSV4STD {
					r := new(dns.A)
					r.Hdr = dns.RR_Header{Name: config.host + ".", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}
					r.A = tw.na.IP
					rr4std = append(rr4std, r)
					numRR++
				}

				// if the twistee is using a non standard port then add the encoded port info to DNS
				if t == DNSV4NON && tw.dnsType == DNSV4NON {
					r := new(dns.A)
					r.Hdr = dns.RR_Header{Name: "nonstd." + config.host + ".", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}
					r.A = tw.na.IP
					rr4non = append(rr4non, r)
					numRR++
					r = new(dns.A)
					r.Hdr = dns.RR_Header{Name: "nonstd." + config.host + ".", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}
					r.A = tw.nonstdIP
					rr4non = append(rr4non, r)
					numRR++
				}
			}
			if t == DNSV6STD || t == DNSV6NON {
				if t == DNSV6STD && tw.dnsType == DNSV6STD {
					r := new(dns.AAAA)
					r.Hdr = dns.RR_Header{Name: config.host + ".", Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60}
					r.AAAA = tw.na.IP
					rr6std = append(rr6std, r)
					numRR++
				}
				// if the twistee is using a non standard port then add the encoded port info to DNS
				if t == DNSV6NON && tw.dnsType == DNSV6NON {
					r := new(dns.AAAA)
					r.Hdr = dns.RR_Header{Name: "nonstd." + config.host + ".", Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60}
					r.AAAA = tw.na.IP
					rr6non = append(rr6non, r)
					numRR++
					r = new(dns.AAAA)
					r.Hdr = dns.RR_Header{Name: "nonstd." + config.host + ".", Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60}
					r.AAAA = tw.nonstdIP
					rr6non = append(rr6non, r)
					numRR++
				}
			}

		}

	}

	s.mtx.RUnlock()

	latest.mtx.Lock()

	for t := range []int{DNSV4STD, DNSV4NON, DNSV6STD, DNSV6NON} {
		switch t {
		case DNSV4STD:
			latest.ipv4std = rr4std
		case DNSV4NON:
			latest.ipv4non = rr4non
		case DNSV6STD:
			latest.ipv6std = rr6std
		case DNSV6NON:
			latest.ipv6non = rr6non
		}
	}

	latest.mtx.Unlock()

	if config.debug {
		log.Printf("debug - DNS update complete - rr4std: %v rr4non: %v rr6std: %v rr6non: %v\n", len(rr4std), len(rr4non), len(rr6std), len(rr6non))
	}
}

// handleDNSStd processes a DNS request from remote client and returns
// a list of current ip addresses that the crawlers consider current.
// This function returns addresses that use the standard port
func handleDNSStd(w dns.ResponseWriter, r *dns.Msg) {

	m := &dns.Msg{MsgHdr: dns.MsgHdr{
		Authoritative:      true,
		RecursionAvailable: false,
	}}
	m.SetReply(r)

	var qtype string

	switch r.Question[0].Qtype {
	case dns.TypeA:
		latest.mtx.RLock()
		m.Answer = getv4stdRR()
		latest.mtx.RUnlock()
		qtype = "A"
	case dns.TypeAAAA:
		latest.mtx.RLock()
		m.Answer = getv6stdRR()
		latest.mtx.RUnlock()
		qtype = "AAAA"
	default:
		// return no answer to all other queries

	}
	if config.verbose {
		log.Printf("status - DNS response Type: standard  To IP: %s  Query Type: %s\n", w.RemoteAddr().String(), qtype)
	}

	// FIXME - add stats and query counts
	w.WriteMsg(m)
}

// handleDNSNon processes a DNS request from remote client and returns
// a list of current ip addresses that the crawlers consider current.
// This function returns addresses that use the non standard port
func handleDNSNon(w dns.ResponseWriter, r *dns.Msg) {

	m := &dns.Msg{MsgHdr: dns.MsgHdr{
		Authoritative:      true,
		RecursionAvailable: false,
	}}
	m.SetReply(r)

	var qtype string

	switch r.Question[0].Qtype {
	case dns.TypeA:
		latest.mtx.RLock()
		m.Answer = getv4nonRR()
		latest.mtx.RUnlock()
		qtype = "A"
	case dns.TypeAAAA:
		latest.mtx.RLock()
		m.Answer = getv6nonRR()
		latest.mtx.RUnlock()
		qtype = "AAAA"
	default:
		// return no answer to all other queries

	}
	if config.verbose {
		log.Printf("status - DNS response Type: non-standard  To IP: %s  Query Type: %s\n", w.RemoteAddr().String(), qtype)
	}

	// FIXME - add stats and query counts
	w.WriteMsg(m)
}

// serve starts the requested DNS server listening on the requested port
func serve(net, port string) {
	server := &dns.Server{Addr: ":" + port, Net: net, TsigSecret: nil}
	if err := server.ListenAndServe(); err != nil {
		log.Printf("Failed to setup the "+net+" server: %v\n", err)
	}
}

/*

 */
