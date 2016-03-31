package main

import (
	"github.com/PreetamJinka/sflow"
	"github.com/PreetamJinka/udpchan"

	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func main() {
	outbound := flag.String("outbound", "localhost:6343", "Address of collector")
	flag.Parse()

	ip := getIP()

	ch, err := udpchan.Connect(*outbound)
	if err != nil {
		log.Fatal(err)
	}

	buf := &bytes.Buffer{}

	enc := sflow.NewEncoder(ip, 23000, 100)

	for _ = range time.Tick(time.Second * 5) {
		stats := []sflow.Record{}

		cpu, err := getCpuStats()
		if err == nil {
			stats = append(stats, cpu)
		}

		mem, err := getMemStats()
		if err == nil {
			stats = append(stats, mem)
		}

		cs := &sflow.CounterSample{}
		cs.Records = stats

		err = enc.Encode(buf, []sflow.Sample{cs})

		if err == nil {
			ch <- buf.Bytes()
			log.Println("Sent a datagram")
		} else {
			log.Println(err)
		}

		buf.Reset()
	}
}

func getCpuStats() (*sflow.HostCPUCounters, error) {
	loadAvgBytes, err := ioutil.ReadFile("/proc/loadavg")
	if err != nil {
		return nil, err
	}

	c := sflow.HostCPUCounters{}

	fmt.Sscanf(string(loadAvgBytes), "%f %f %f %d/%d", &c.Load1m, &c.Load5m, &c.Load15m,
		&c.ProcessesRunning, &c.ProcessesTotal)

	c.NumCPU = uint32(runtime.NumCPU())

	uptimeBytes, err := ioutil.ReadFile("/proc/uptime")
	if err != nil {
		return nil, err
	}
	fmt.Sscanf(string(uptimeBytes), "%d", &c.Uptime)

	cpuStatBytes, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return nil, err
	}
	fmt.Sscanf(string(cpuStatBytes), "cpu %d %d %d %d %d %d %d", &c.CPUUser, &c.CPUNice, &c.CPUSys, &c.CPUIdle,
		&c.CPUWio, &c.CPUIntr, &c.CPUSoftIntr)

	return &c, nil
}

func getMemStats() (*sflow.HostMemoryCounters, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, err
	}

	m := sflow.HostMemoryCounters{}

	r := bufio.NewReader(f)
	for line, err := r.ReadString('\n'); err == nil; line, err = r.ReadString('\n') {
		lineStr := string(line)
		parts := strings.Split(lineStr, ":")
		switch parts[0] {
		case "MemTotal":
			m.Total = meminfoHelper(parts[1])
		case "MemFree":
			m.Free = meminfoHelper(parts[1])
		case "Buffers":
			m.Buffers = meminfoHelper(parts[1])
		case "Cached":
			m.Cached = meminfoHelper(parts[1])
		case "SwapTotal":
			m.SwapTotal = meminfoHelper(parts[1])
		case "SwapFree":
			m.SwapFree = meminfoHelper(parts[1])
		}
	}

	return &m, nil
}

func meminfoHelper(s string) uint64 {
	parts := strings.Split(strings.TrimSpace(s), " ")
	n, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0
	}
	return uint64(n) * 1024
}

func getIP() net.IP {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return nil
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			return ip
		}
	}

	return net.IPv6loopback

}
