package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/wire"
	"log"
	"os"
	"strconv"
)

// JNetwork is the exported struct that is read from the network file
type JNetwork struct {
	Name       string
	Desc       string
	SeederType string
	Secret     string
	Remote     string
	Id         string
	Port       uint16
	Pver       uint32
	DNSName    string
	TTL        uint32
	Seeder1    string
	Seeder2    string
	Seeder3    string
}

func createNetFile() {
	// create a standard json template file that can be loaded into the app

	// create a struct to encode with json
	jnw := &JNetwork{
		Id:         "0xabcdef01",
		Port:       1234,
		Pver:       70001,
		TTL:        600,
		DNSName:    "seeder.example.com",
		Name:       "SeederNet",
		Desc:       "Description of SeederNet",
		SeederType: "Combined",
		Secret:     "32bYTesoFSECretThAtiSASecrET!!",
		Remote:     "http://dnsserver.example.com:1234/updatedns",
		Seeder1:    "seeder1.example.com",
		Seeder2:    "seed1.bob.com",
		Seeder3:    "seed2.example.com",
	}

	f, err := os.Create("dnsseeder.json")
	if err != nil {
		log.Printf("error creating template file: %v\n", err)
	}
	defer f.Close()

	j, jerr := json.MarshalIndent(jnw, "", " ")
	if jerr != nil {
		log.Printf("error parsing json: %v\n", err)
	}
	_, ferr := f.Write(j)
	if ferr != nil {
		log.Printf("error writing to template file: %v\n", err)
	}
}

func loadNetwork(fName string) (*dnsseeder, error) {
	nwFile, err := os.Open(fName)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error reading network file: %v", err))
	}

	defer nwFile.Close()

	var jnw JNetwork

	jsonParser := json.NewDecoder(nwFile)
	if err = jsonParser.Decode(&jnw); err != nil {
		return nil, errors.New(fmt.Sprintf("Error decoding network file: %v", err))
	}

	return initNetwork(jnw)
}

func initNetwork(jnw JNetwork) (*dnsseeder, error) {

	if jnw.Port == 0 {
		return nil, errors.New(fmt.Sprintf("Invalid port supplied: %v", jnw.Port))
	}

	if jnw.DNSName == "" {
		return nil, errors.New(fmt.Sprintf("No DNS Hostname supplied"))
	}

	// we only need a secret if we are a crawler or dns type
	if needSecret := convertSeederType(jnw.SeederType); needSecret != typeCombined {
		if ok := checkBlockSize(jnw.Secret); ok != true {
			return nil, errors.New(fmt.Sprintf("shared secret must be either 16, 24 or 32 bytes long. currently: %v", len(jnw.Secret)))
		}
	}
	if _, ok := config.seeders[jnw.Name]; ok {
		return nil, errors.New(fmt.Sprintf("Name already exists from previous file - %s", jnw.Name))
	}

	// init the seeder
	seeder := &dnsseeder{}
	seeder.theList = make(map[string]*node)
	seeder.port = jnw.Port
	seeder.pver = jnw.Pver
	seeder.ttl = jnw.TTL
	seeder.name = jnw.Name
	seeder.desc = jnw.Desc
	seeder.dnsHost = jnw.DNSName
	seeder.seederType = convertSeederType(jnw.SeederType)
	seeder.secret = jnw.Secret
	seeder.remote = jnw.Remote

	// conver the network magic number to a Uint32
	t1, err := strconv.ParseUint(jnw.Id, 0, 32)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error converting Network Magic number: %v", err))
	}
	seeder.id = wire.BitcoinNet(t1)

	// load the seeder dns
	seeder.seeders = make([]string, 3)
	seeder.seeders[0] = jnw.Seeder1
	seeder.seeders[1] = jnw.Seeder2
	seeder.seeders[2] = jnw.Seeder3

	// add some checks to the start & delay values to keep them sane
	seeder.maxStart = []uint32{20, 20, 20, 30}
	seeder.delay = []int64{210, 789, 234, 1876}
	seeder.maxSize = 1250

	// initialize the stats counters
	seeder.counts.NdStatus = make([]uint32, maxStatusTypes)
	seeder.counts.NdStarts = make([]uint32, maxStatusTypes)
	seeder.counts.DNSCounts = make([]uint32, maxDNSTypes)

	// some sanity checks on the loaded config options
	if seeder.ttl < 60 {
		seeder.ttl = 60
	}
	// check for duplicates
	for _, v := range config.seeders {
		if v.id == seeder.id {
			return nil, errors.New(fmt.Sprintf("Duplicate Magic id. Already loaded for %s so can not be used for %s", v.id, v.name, seeder.name))
		}
		if v.dnsHost == seeder.dnsHost {
			return nil, errors.New(fmt.Sprintf("Duplicate DNS names. Already loaded %s for %s so can not be used for %s", v.dnsHost, v.name, seeder.name))
		}
	}

	return seeder, nil
}

func convertSeederType(seederType string) uint32 {
	switch seederType {
	case "Crawl":
		return typeCrawl
	case "DNS":
		return typeDNS
	default:
		return typeCombined
	}
}

func checkBlockSize(secret string) bool {
	s := len(secret)
	if s == 16 || s == 24 || s == 32 {
		return true
	}
	return false
}

/*

 */
