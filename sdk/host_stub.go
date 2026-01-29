//go:build !tinygo

package sdk

func hostGetConfig(_ uint32, _ uint32) int32 {
	return hostErrNotFound
}

func hostLog(_ uint32, _ uint32, _ uint32) {}

func hostSubmitResult(_ uint32, _ uint32) int32 {
	return hostErrNotFound
}

func hostHTTPRequest(_ uint32, _ uint32, _ uint32, _ uint32) int32 {
	return hostErrNotFound
}

func hostTCPConnect(_ uint32, _ uint32, _ uint32, _ uint32) int32 {
	return hostErrNotFound
}

func hostTCPRead(_ uint32, _ uint32, _ uint32, _ uint32) int32 {
	return hostErrNotFound
}

func hostTCPWrite(_ uint32, _ uint32, _ uint32, _ uint32) int32 {
	return hostErrNotFound
}

func hostTCPClose(_ uint32) int32 {
	return hostErrNotFound
}

func hostUDPSendTo(_ uint32, _ uint32, _ uint32, _ uint32, _ uint32, _ uint32) int32 {
	return hostErrNotFound
}
