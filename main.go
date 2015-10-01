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

// NodeCounts holds various statistics about the running system for use in html templates
type NodeCounts struct {
	NdStatus  []uint32     // number of nodes at each of the 4 statuses - RG, CG, WG, NG
	NdStarts  []uint32     // number of crawles started last startcrawlers run
	DNSCounts []uint32     // number of dns requests for each dns type - dnsV4Std, dnsV4Non, dnsV6Std, dnsV6Non
	mtx       sync.RWMutex // protect the structures
}

// configData holds information on the application
type configData struct {
	dnsUnknown uint64                // the number of dns requests for we are not configured to handle
	uptime     time.Time             // application start time
	port       string                // port for the dns server to listen on
	http       string                // port for the web server to listen on
	version    string                // application version
	seeders    map[string]*dnsseeder // holds a pointer to all the current seeders
	smtx       sync.RWMutex          // protect the seeders map
	order      []string              // the order of loading the netfiles so we can display in this order
	dns        map[string][]dns.RR   // holds details of all the currently served dns records
	dnsmtx     sync.RWMutex          // protect the dns map
	verbose    bool                  // verbose output cmdline option
	debug      bool                  // debug cmdline option
	stats      bool                  // stats cmdline option
}

var config configData
var netfile string

func main() {

	var j bool

	config.version = "0.9.1"
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
	config.order = []string{}

	for _, nwFile := range netwFiles {
		nnw, err := loadNetwork(nwFile)
		if err != nil {
			fmt.Printf("Error loading data from netfile %s - %v\n", nwFile, err)
			os.Exit(1)
		}
		if nnw != nil {
			config.seeders[nnw.name] = nnw
			config.order = append(config.order, nnw.name)
		}
	}

	if config.debug == true {
		config.verbose = true
		config.stats = true
	}
	if config.verbose == true {
		config.stats = true
	}

	for _, v := range config.seeders {
		log.Printf("status - system is configured for network: %s\n", v.name)
	}

	if config.verbose == false {
		log.Printf("status - Running in quiet mode with limited output produced\n")
	}

	// start the web interface if we want it running
	if config.http != "" {
		go startHTTP(config.http)
	}

	// start dns server
	dns.HandleFunc(".", handleDNS)
	go serve("udp", config.port)
	//go serve("tcp", config.port)

	var wg sync.WaitGroup

	done := make(chan struct{})
	// start a goroutine for each seeder
	for _, s := range config.seeders {
		wg.Add(1)
		go s.runSeeder(done, &wg)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// block until a signal is received
	fmt.Println("\nShutting down on signal:", <-sig)

	// FIXME - call dns server.Shutdown()

	// close the done channel to signal to all seeders to shutdown
	// and wait for them to exit
	close(done)
	wg.Wait()
	fmt.Printf("\nProgram exiting. Bye\n")
}

// updateNodeCounts runs in a goroutine and updates the global stats with the latest
// counts from a startCrawlers run
func updateNodeCounts(s *dnsseeder, tcount uint32, started, totals []uint32) {
	s.counts.mtx.Lock()

	for st := range []int{statusRG, statusCG, statusWG, statusNG} {
		if config.stats {
			log.Printf("%s: started crawler: %s total: %v started: %v\n", s.name, status2str(uint32(st)), totals[st], started[st])
		}

		// update the stats counters
		s.counts.NdStatus[st] = totals[st]
		s.counts.NdStarts[st] = started[st]
	}

	if config.stats {
		log.Printf("%s: crawlers started. total nodes: %d\n", s.name, tcount)
	}
	s.counts.mtx.Unlock()
}

// status2str will return the string description of the status
func status2str(status uint32) string {
	switch status {
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

/*

 */
