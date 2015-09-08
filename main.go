/*

 */
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/miekg/dns"
)

// ndCounts holds various statistics about the running system
type NodeCounts struct {
	NdStatus  []uint32     // number of nodes at each of the 4 statuses - RG, CG, WG, NG
	NdStarts  []uint32     // number of crawles started last startcrawlers run
	DNSCounts []uint32     // number of dns requests for each dns type - dnsV4Std, dnsV4Non, dnsV6Std, dnsV6Non
	mtx       sync.RWMutex // protect the structures
}

// configData holds information on the application
type configData struct {
	uptime     time.Time             // application start time
	port       string                // port for the dns server to listen on
	http       string                // port for the web server to listen on
	version    string                // application version
	verbose    bool                  // verbose output cmdline option
	debug      bool                  // debug cmdline option
	stats      bool                  // stats cmdline option
	seeders    map[string]*dnsseeder // holds a pointer to all the current seeders
	smtx       sync.RWMutex          // protect the seeders map
	dns        map[string][]dns.RR   // holds details of all the currently served dns records
	dnsmtx     sync.RWMutex          // protect the dns map
	dnsUnknown uint64                // the number of dns requests for we are not configured to handle
}

var config configData
var netfile string

func main() {

	var j bool

	// FIXME - update with git hash during build
	config.version = "0.6.0"
	config.uptime = time.Now()

	flag.StringVar(&netfile, "netfile", "", "List of json config files to load")
	flag.StringVar(&config.port, "p", "8053", "DNS Port to listen on")
	flag.StringVar(&config.http, "w", "", "Web Port to listen on. No port specified & no web server running")
	flag.BoolVar(&j, "j", false, "Write network template file (dnsseeder.json) and exit")
	flag.BoolVar(&config.verbose, "v", false, "Display verbose output")
	flag.BoolVar(&config.debug, "d", false, "Display debug output")
	flag.BoolVar(&config.stats, "s", false, "Display stats output")
	flag.Parse()

	if j == true {
		createNetFile()
		fmt.Printf("Template file has been created\n")
		os.Exit(0)
	}

	// configure the network options so we can start crawling
	netwFiles := strings.Split(netfile, ",")
	if len(netwFiles) == 0 {
		fmt.Printf("Error - No filenames specified. Please add -net=<file[, file2]> to load these files\n")
		os.Exit(1)
	}

	config.seeders = make(map[string]*dnsseeder)
	config.dns = make(map[string][]dns.RR)

	for _, nwFile := range netwFiles {
		nnw, err := loadNetwork(nwFile)
		if err != nil {
			fmt.Printf("Error loading data from netfile %s - %v\n", nwFile, err)
			os.Exit(1)
		}
		if nnw != nil {
			// FIXME - lock this
			config.seeders[nnw.name] = nnw
		}
	}

	if config.debug == true {
		config.verbose = true
		config.stats = true
	}
	if config.verbose == true {
		config.stats = true
	}

	if config.verbose == false {
		log.Printf("status - Running in quiet mode with limited output produced\n")
	} else {
		for _, v := range config.seeders {
			log.Printf("status - system is configured for network: %s\n", v.name)
		}
	}

	// start the web interface if we want it running
	if config.http != "" {
		go startHTTP(config.http)
	}

	// start dns server
	dns.HandleFunc(".", handleDNS)
	go serve("udp", config.port)
	//go serve("tcp", config.port)

	// seed the seeder with some ip addresses
	for _, s := range config.seeders {
		s.initCrawlers()
		s.startCrawlers()
	}

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// extract good dns records from all nodes on regular basis
	dnsChan := time.NewTicker(time.Second * dnsDelay).C
	// used to start crawlers on a regular basis
	crawlChan := time.NewTicker(time.Second * crawlDelay).C
	// used to remove old statusNG nodes that have reached fail count
	auditChan := time.NewTicker(time.Minute * auditDelay).C

	dowhile := true
	for dowhile == true {
		select {
		case <-sig:
			dowhile = false
		case <-auditChan:
			if config.debug {
				log.Printf("debug - Audit nodes timer triggered\n")
			}
			for _, s := range config.seeders {
				// FIXME goroutines for these
				s.auditNodes()
			}
		case <-dnsChan:
			if config.debug {
				log.Printf("debug - DNS - Updating latest ip addresses timer triggered\n")
			}
			for _, s := range config.seeders {
				s.loadDNS()
			}
		case <-crawlChan:
			if config.debug {
				log.Printf("debug - Start crawlers timer triggered\n")
			}
			for _, s := range config.seeders {
				s.startCrawlers()
			}
		}
	}
	// FIXME - call dns server.Shutdown()
	fmt.Printf("\nProgram exiting. Bye\n")
}

// updateNodeCounts runs in a goroutine and updates the global stats with the latest
// counts from a startCrawlers run
func updateNodeCounts(s *dnsseeder, status, total, started uint32) {
	// update the stats counters
	s.counts.mtx.Lock()
	s.counts.NdStatus[status] = total
	s.counts.NdStarts[status] = started
	s.counts.mtx.Unlock()
}

// updateDNSCounts runs in a goroutine and updates the global stats for the number of DNS requests
func updateDNSCounts(name, qtype string) {
	var ndType uint32
	var counted bool

	nonstd := strings.HasPrefix(name, "nonstd.")

	switch qtype {
	case "A":
		if nonstd {
			ndType = dnsV4Non
		} else {
			ndType = dnsV4Std
		}
	case "AAAA":
		if nonstd {
			ndType = dnsV6Non
		} else {
			ndType = dnsV6Std
		}
	default:
		ndType = dnsInvalid
	}

	// for DNS requests we do not have a reference to a seeder so we have to find it
	for _, s := range config.seeders {
		s.counts.mtx.Lock()

		if name == s.dnsHost+"." || name == "nonstd."+s.dnsHost+"." {
			s.counts.DNSCounts[ndType]++
			counted = true
		}
		s.counts.mtx.Unlock()
	}
	if counted != true {
		atomic.AddUint64(&config.dnsUnknown, 1)
	}
}

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
