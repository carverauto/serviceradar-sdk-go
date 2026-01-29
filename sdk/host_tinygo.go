//go:build tinygo

package sdk

//go:wasmimport env get_config
func hostGetConfig(ptr uint32, size uint32) int32

//go:wasmimport env log
func hostLog(level uint32, ptr uint32, size uint32)

//go:wasmimport env submit_result
func hostSubmitResult(ptr uint32, size uint32) int32

//go:wasmimport env http_request
func hostHTTPRequest(reqPtr uint32, reqLen uint32, respPtr uint32, respLen uint32) int32

//go:wasmimport env tcp_connect
func hostTCPConnect(addrPtr uint32, addrLen uint32, port uint32, timeoutMS uint32) int32

//go:wasmimport env tcp_read
func hostTCPRead(handle uint32, bufPtr uint32, bufLen uint32, timeoutMS uint32) int32

//go:wasmimport env tcp_write
func hostTCPWrite(handle uint32, bufPtr uint32, bufLen uint32, timeoutMS uint32) int32

//go:wasmimport env tcp_close
func hostTCPClose(handle uint32) int32

//go:wasmimport env udp_sendto
func hostUDPSendTo(addrPtr uint32, addrLen uint32, port uint32, bufPtr uint32, bufLen uint32, timeoutMS uint32) int32
