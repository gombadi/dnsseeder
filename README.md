# dnsseeder
Go Language dns seeder for Networks that use Bitcoin technology such as the [Twister P2P network](http://twister.net.co/) and the Bitcoin networks.

It is based on the original c++ seeders created for the Bitcoin network and copied to other similar networks.

## Features

* Supports multiple networks. You can run multiple seeders off one ip address.
* Uses Go Language so it can easily be compiled and run on multiple platforms.
* Minimal resource requirements. Will easily seed multiple networks on a Raspberry Pi 1 Mobel B+
* Restricts the number of addresses accepted from any one node.
* Cycle through working nodes to keep the active list fresh
* Reduces bandwidth usage on nodes if it has many working nodes already in the system.
* Ability to generate and edit your own seeder config file to support new networks.

### Planned features

* Support remote crawlers. Run the DNS seeder on one system and the crawlers on a different system.


Also see the associated utility to display information about [non-standard ip addresses](https://github.com/gombadi/nonstd/)



## Installing

Simply use go get to download the code:

    $ go get github.com/gombadi/dnsseeder

Dependencies are handled by the Go vendor directory.
Note: This means the codebase requires Go 1.5 or higher and use of `GO15VENDOREXPERIMENT=1`

There seem to be some issues with using `go get` so a process that works for me on Unix based systems is -

```
cd ${HOME}
mkdir -p go/src/github.com/gombadi
cd go/src/github.com/gombadi
git clone the repo into this directory
cd dnsseeder
go install

```
The binary will then be available in `${HOME}/go/bin`


## Usage

First, choose one seed domain name per network that you want to seed, as well as one nameserver domain name.  These can be any domain that you control.  For this example, we'll use `btc.seed.example.com` as your seed domain name and `ns.seed.example.net` as your nameserver domain name.  For each network that you want to seed, set the `"DNSName"` JSON field in its config file to the seed domain name that you picked for that network, e.g. `"DNSName": "btc.seed.example.com",`.  Optionally, fill in any number of IP addresses of nodes running on that network into the `"InitialIP"` field, e.g. `"InitialIP": "127.0.0.1,1.2.3.4",`.

Then, run the seeder:

    $ dnsseeder -v -netfile <filename1,filename2>

An easy way to run the program is with tmux or screen. This enables you to log out and leave the program running.

If you want to be able to view the web interface then add `-w port` for the web server to listen on. If this is not provided then no web interface will be available. With the web site running you can then access the site by http://localhost:port/summary

**NOTE -** For security reasons the web server will only listen on localhost so you will need to either use an ssh tunnel or proxy requests via a web server like Nginx or Apache.

```

Command line Options:
-netfile comma seperated list of json network config files to load
-j write a sample network config file in json format and exit.
-p port to listen on for DNS requests
-d Produce debug output
-v Produce verbose output
-w Port to listen on for Web Interface

```

An easy way to run the program is with the following script. Change to suit your system.

```

#!/bin/bash

LOGDIR=${HOME}/goseederlogs/

mkdir -p ${LOGDIR}

gzip ${LOGDIR}/*.log

cd
echo
echo "======= Run the Go Language dnsseed ======="
echo
${HOME}/go/bin/dnsseeder -p <dns.port.to.listen.on> -v -w 8880 -netfile ${1} 2>&1 | tee ${LOGDIR}/$(date +%F-%s)-goseeder.log


```

Once your seeder is running, set up an `A` or `AAAA` DNS record on your nameserver domain name, pointing to the public IP address of the machine running your seeder.  Then set up an `NS` DNS record on each seed domain name, pointing to your nameserver domain name.

## RUNNING AS NON-ROOT

Typically, you'll need root privileges to listen to port 53 (name service).  Some potential solutions:

### iptables

One solution is using an iptables rule (Linux only) to redirect it to
a non-privileged port:

```
$ sudo iptables -t nat -A PREROUTING -p udp --dport 53 -j REDIRECT --to-port 5353
$ sudo iptables -t nat -A PREROUTING -p tcp --dport 53 -j REDIRECT --to-port 5353
```

If properly configured, this will allow you to run dnsseeder in userspace, using
the `-p 5353` option.

### setcap

On Linux, another solution is running the following command to authorize dnsseeder to bind to privileged ports.

```
$ sudo setcap 'cap_net_bind_service=+ep' ${HOME}/go/bin/dnsseeder
```

## License

For the DNS library license see https://github.com/miekg/dns

For the bitcoin library license see https://github.com/btcsuite/btcd


