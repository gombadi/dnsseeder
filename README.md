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
Note: This means the codebase requires Go 1.5 or higher and use of GO15VENDOREXPERIMENT=1

There seem to be some issues with using go get so a process that works for me on Unix based systems is -

```
cd ${HOME}
mkdir -p go/src/github.com/gombadi
cd go/src/github.com/gombadi
git clone the repo into this directory
cd dnsseeder
go install

```
The binary will then be available in ${HOME}/go/bin


## Usage

    $ dnsseeder -v -netfile <filename1,filename2>

An easy way to run the program is with tmux or screen. This enables you to log out and leave the program running.

If you want to be able to view the web interface then add -w port for the web server to listen on. If this is not provided then no web interface will be available. With the web site running you can then access the site by http://localhost:port/summary

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


## RUNNING AS NON-ROOT

Typically, you'll need root privileges to listen to port 53 (name service).

One solution is using an iptables rule (Linux only) to redirect it to
a non-privileged port:

$ iptables -t nat -A PREROUTING -p udp --dport 53 -j REDIRECT --to-port 5353

If properly configured, this will allow you to run dnsseeder in userspace, using
the -p 5353 option.


## License

For the DNS library license see https://github.com/miekg/dns

For the bitcoin library license see https://github.com/btcsuite/btcd


