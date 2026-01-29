package sdk

// Execute runs the plugin function and submits its result.
func Execute(fn func() Result) {
	result := fn()
	if result.Status == "" {
		result.Status = StatusUnknown
	}
	if result.Summary == "" {
		result.Summary = string(result.Status)
	}
	payload, err := result.Serialize()
	if err != nil {
		Log.Error("failed to serialize result")
		return
	}
	if err := SubmitResult(payload); err != nil {
		Log.Error("failed to submit result")
	}
}

// SubmitResult sends a serialized result payload to the host.
func SubmitResult(payload []byte) error {
	if len(payload) == 0 {
		return HostError{Code: hostErrInvalid, Op: "submit_result"}
	}
	res := hostSubmitResult(ptrFromBytes(payload), uint32(len(payload)))
	return hostErr(res, "submit_result")
}
