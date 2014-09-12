package main

import (
	"github.com/PreetamJinka/sflow-go"
	"github.com/PreetamJinka/udpchan"

	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
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

	ch, _ := udpchan.Connect(*outbound)
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

		if err == nil {
			ch <- sflow.Encode(ip, 1, 0,
				1, 1, 1, 1, stats)
		} else {
			fmt.Println(err)
		}
	}
}

func getCpuStats() (*sflow.HostCpuCounters, error) {
	loadAvgBytes, err := ioutil.ReadFile("/proc/loadavg")
	if err != nil {
		return nil, err
	}

	c := sflow.HostCpuCounters{}

	fmt.Sscanf(string(loadAvgBytes), "%f %f %f %d/%d", &c.Load1m, &c.Load5m, &c.Load15m,
		&c.ProcsRunning, &c.ProcsTotal)

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
	fmt.Sscanf(string(cpuStatBytes), "cpu %d %d %d %d %d %d %d", &c.CpuUser, &c.CpuNice, &c.CpuSys, &c.CpuIdle,
		&c.CpuWio, &c.CpuIntr, &c.CpuSoftIntr)

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
