package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-ping/ping"
	"github.com/miekg/dns"
	"github.com/txn2/txeh"
)

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func getAnswer(domain string, dnsserverName string, dnsserverPort string) (ans []dns.RR) {

	dnsClient := new(dns.Client)

	dnsMsg := new(dns.Msg)
	dnsMsg.SetQuestion(dns.Fqdn(domain), dns.TypeA)
	dnsMsg.RecursionDesired = true

	r, _, err := dnsClient.Exchange(dnsMsg, net.JoinHostPort(dnsserverName, dnsserverPort))
	if r == nil {
		log.Fatalln(err.Error())
	}

	if r.Rcode != dns.RcodeSuccess {
		log.Fatalf(" *** invalid answer name %s\n", domain)
	}
	return r.Answer
}

func updateHosts(domain string, answers []dns.RR, hosts *txeh.Hosts, fallbackDomain string, dnsserverName string, dnsserverPort string, rangeCIDR bool) {
	for index, a := range answers {
		if a.Header().Rrtype != dns.TypeA {
			continue
		}
		ip := a.(*dns.A).A.String()

		if checkICMP(ip) {
			hosts.AddHost(ip, domain)
			return
		}
		if index == len(answers)-1 && !checkICMP(ip) {
			if fallbackDomain != "" {
				// fallback only loop once
				fallbackAnswers := getAnswer(fallbackDomain, dnsserverName, dnsserverPort)
				updateHosts(domain, fallbackAnswers, hosts, "", dnsserverName, dnsserverPort, rangeCIDR)
				if rangeCIDR {
					fmt.Println("-b and -l both exist, ignore -l")
				}
				return
			}

			if rangeCIDR {

				mask := net.CIDRMask(24, 32) // subnet 24
				ipNet := &net.IPNet{
					IP:   net.ParseIP(ip).Mask(mask),
					Mask: mask,
				}

				for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); inc(ip) {
					// ip range 1-254
					if ip[3] == 0 || ip[3] == 255 {
						continue
					}
					if checkICMP(ip.String()) {
						hosts.AddHost(ip.String(), domain)
						return
					}
				}
			}
		}
	}
}

func checkICMP(ip string) (icmpReachable bool) {

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
	mosdnsStyle := flag.Bool("m", false, "save to hosts.mosdns as mosdns style")
	loopCIDR := flag.Bool("l", false, "loop /24 cidr (1-254)")
	hostsFile := flag.String("f", "hosts", "output hosts file")
	fallbackDomain := flag.String("b", "", "fallback domain when all ips are unreachable")
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

	var server string
	if *udpServer != "" {
		server = *udpServer
	} else {
		config, _ := dns.ClientConfigFromFile("/etc/resolv.conf")
		server = config.Servers[0]
	}

	var port int
	if *udpPort != 0 {
		port = *udpPort
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
		defer func(file *os.File) {
			err := file.Close()
			if err != nil {
				log.Fatalf(" *** failed to open/close file %s\n", err)
			}
		}(file)
	}
	hosts, err := txeh.NewHosts(&txeh.HostsConfig{
		ReadFilePath:  hfPath,
		WriteFilePath: hfPath,
	})

	if err != nil {
		panic(err)
	}

	// get answers
	answers := getAnswer(*domain, server, strconv.Itoa(port))

	// update hosts
	updateHosts(*domain, answers, hosts, *fallbackDomain, server, strconv.Itoa(port), *loopCIDR)

	hfData := hosts.RenderHostsFile()
	fmt.Print(hfData)

	if *update {
		hosts.Save()

		if *mosdnsStyle {
			// write mosdnsStyleHosts (domain ip) to hfPath+".mosdns" file
			oldMosdnsFile, _ := os.OpenFile(hfPath+".mosdns", os.O_RDWR|os.O_CREATE, 0644)
			data, _ := io.ReadAll(oldMosdnsFile)
			oldMosdnsFile.Close()
			oldMosdnsHosts := string(data)

			oldMosdnsHostsLines := strings.Split(oldMosdnsHosts, "\n")
			hfls := hosts.GetHostFileLines()
			mosdnsStyleHosts := ""
			updated := false
			for _, hfl := range *hfls {
				for _, hostname := range hfl.Hostnames {
					if hostname == *domain {
						for _, line := range oldMosdnsHostsLines {

							line = strings.TrimSpace(line)
							if strings.HasPrefix(line, *domain) {
								if updated {
									continue
								} else {
									mosdnsStyleHosts += fmt.Sprintf("%s %s\n", hostname, hfl.Address)
									updated = true
								}
							} else {
								if line != "" {
									mosdnsStyleHosts += line + "\n"
								} else {
									if updated {
										continue
									} else {
										mosdnsStyleHosts += fmt.Sprintf("%s %s\n", hostname, hfl.Address)
										updated = true
									}
								}
							}
						}
					}
				}
			}

			// overwrite the file
			writeFile, _ := os.OpenFile(hfPath+".mosdns", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
			writeFile.WriteString(mosdnsStyleHosts)
		}
	}
}
