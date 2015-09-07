# dnsseeder
Go Language dns seeder for Networks that use Bitcoin technology such as the [Twister P2P network](http://twister.net.co/) and the Bitcoin networks.

It is based on the original c++ seeders created for the Bitcoin network and copied to other similar networks.

This application can seed one or more networks on the same ip address. At the moment there are config files for the Twister and Bitcoin networks. You use the -netfile commandline option to specify one or more comma seperated filenames to load the network configuration for that network.

You can use the -j option to produce a sample json network config file (dnsseeder.json) in the current directory and then edit the file to seed your own network.

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


