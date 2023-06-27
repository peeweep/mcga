package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/go-ping/ping"
	"github.com/miekg/dns"
	"github.com/txn2/txeh"
)

type DNSSERVER struct {
	server string
	port   int
}

func check_ping(ip string) (icmp_reachable bool) {

	pinger, err := ping.NewPinger(ip)
	if err != nil {
		log.Fatal(err)
	}

	pinger.Count = 5
	pinger.Timeout = time.Second * 1

	err = pinger.Run()
	if err != nil {
		log.Fatal(err)
	}

	stats := pinger.Statistics()
	if stats.PacketsSent == stats.PacketsRecv {
		return true
	}
	return false
}

var Version string

func main() {
	domain := flag.String("d", "", "domain name")
	udpServer := flag.String("s", "", "dns server (use /etc/resolv.conf if not specified)")
	udpPort := flag.Int("p", 53, "dns udp port")
	update := flag.Bool("u", true, "update hosts file")
	hostsFile := flag.String("f", "hosts", "output hosts file")
	version := flag.Bool("v", false, "show version")

	flag.Parse()

	if *version {
		if Version == "" {
			fmt.Printf("Version: Unknown\n")
		} else {
			fmt.Printf("Version: %v\n", Version)
		}
		return
	}

	if *domain == "" && flag.NArg() > 0 {
		*domain = flag.Arg(0)
	}

	if *domain == "" {
		fmt.Println("No domain name specified.")
		return
	}

	var dns_server string
	if *udpServer != "" {
		dns_server = *udpServer
	} else {
		config, _ := dns.ClientConfigFromFile("/etc/resolv.conf")
		dns_server = config.Servers[0]
	}

	var port int
	if *udpPort != 0 {
		port = *udpPort
	}

	dnsc := new(dns.Client)

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(*domain), dns.TypeA)
	m.RecursionDesired = true

	r, _, err := dnsc.Exchange(m, net.JoinHostPort(dns_server, strconv.Itoa(port)))
	if r == nil {
		log.Fatalln(err.Error())
	}

	if r.Rcode != dns.RcodeSuccess {
		log.Fatalf(" *** invalid answer name %s\n", *domain)
	}

	var hfPath string
	if *hostsFile != "" {
		hfPath = *hostsFile
	}

	if *update {
		file, err := os.OpenFile(hfPath, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			log.Fatalf(" *** failed to open file %s\n", err)
			return
		}
		defer file.Close()
	}
	hosts, err := txeh.NewHosts(&txeh.HostsConfig{
		ReadFilePath:  hfPath,
		WriteFilePath: hfPath,
	})

	if err != nil {
		panic(err)
	}
	for _, a := range r.Answer {
		if a.Header().Rrtype != dns.TypeA {
			continue
		}
		ip := a.(*dns.A).A.String()

		if check_ping(ip) {
			hosts.AddHost(ip, *domain)
			break
		}
	}

	hfData := hosts.RenderHostsFile()
	fmt.Println(hfData)

	if *update {
		hosts.Save()
	}
}
