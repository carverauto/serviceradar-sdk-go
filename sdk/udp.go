package sdk

import "time"

// UDPSendTo sends a UDP payload via the host proxy.
func UDPSendTo(host string, port uint16, payload []byte, timeout time.Duration) error {
	addr := []byte(host)
	res := hostUDPSendTo(
		ptrFromBytes(addr),
		uint32(len(addr)),
		uint32(port),
		ptrFromBytes(payload),
		uint32(len(payload)),
		uint32(timeout.Milliseconds()),
	)
	return hostErr(res, "udp_sendto")
}
