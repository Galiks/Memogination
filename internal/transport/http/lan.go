package http

import (
	"net"
	"strings"
)

// DetectLANAddresses enumerates non-loopback IPv4 addresses on all interfaces
// and returns them as "http://<ip>:<port>" strings, deduplicated. If port
// begins with ":" the leading colon is stripped.
func DetectLANAddresses(port string) []string {
	port = strings.TrimPrefix(port, ":")
	seen := map[string]bool{}
	var out []string

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return out
	}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipnet.IP.To4()
		if ip == nil || ip.IsLoopback() {
			continue
		}
		addr := "http://" + ip.String() + ":" + port
		if seen[addr] {
			continue
		}
		seen[addr] = true
		out = append(out, addr)
	}
	return out
}
