package sdk

import (
	"encoding/json"
	"reflect"
	"testing"
)

// The wire documents below are written out by hand from the host contract, so
// a renamed key or schema fails here.

func TestParseCameraSourceResolutionRequest(t *testing.T) {
	config := []byte(`{
		"host": "192.0.2.30",
		"_serviceradar_camera_source_resolution_request": {
			"schema": "serviceradar.camera_source_resolution_request.v1",
			"vendor_camera_id": "camera-vendor-1",
			"vendor_profile_id": "high"
		}
	}`)

	request, ok, err := ParseCameraSourceResolutionRequest(config)
	if err != nil || !ok {
		t.Fatalf("parse: ok=%v err=%v", ok, err)
	}
	if request.VendorCameraID != "camera-vendor-1" || request.VendorProfileID != "high" {
		t.Fatalf("request = %+v", request)
	}

	for _, ordinary := range []string{``, `{}`, `{"host":"192.0.2.30"}`} {
		if _, ok, err := ParseCameraSourceResolutionRequest([]byte(ordinary)); ok || err != nil {
			t.Errorf("ordinary config %q: ok=%v err=%v, want no request", ordinary, ok, err)
		}
	}
}

// A request the plugin cannot act on is an error, never a silent fall through
// to an ordinary inventory run.
func TestParseCameraSourceResolutionRequestRejectsUnusableRequests(t *testing.T) {
	for name, config := range map[string]string{
		"unknown schema": `{"_serviceradar_camera_source_resolution_request":{"schema":"serviceradar.camera_source_resolution_request.v2","vendor_camera_id":"camera-vendor-1"}}`,
		"no camera id":   `{"_serviceradar_camera_source_resolution_request":{"schema":"serviceradar.camera_source_resolution_request.v1","vendor_camera_id":" "}}`,
		"not an object":  `{"_serviceradar_camera_source_resolution_request":"high"}`,
		"config invalid": `[`,
	} {
		if _, ok, err := ParseCameraSourceResolutionRequest([]byte(config)); ok || err == nil {
			t.Errorf("%s: ok=%v err=%v, want an error", name, ok, err)
		}
	}
}

func TestCameraSourceResolutionSerialize(t *testing.T) {
	cases := map[string]struct {
		resolution CameraSourceResolution
		want       map[string]any
	}{
		"address": {
			resolution: CameraSourceResolution{URL: " rtsps://192.0.2.10:7441/aliasInvented0001?enableSrtp "},
			want: map[string]any{
				"schema": "serviceradar.camera_source_resolution.v1",
				"url":    "rtsps://192.0.2.10:7441/aliasInvented0001?enableSrtp",
			},
		},
		"failure": {
			resolution: CameraSourceResolution{FailureClass: CameraSourceFailureNotFound, FailureDetail: "streaming is disabled for this camera"},
			want: map[string]any{
				"schema":         "serviceradar.camera_source_resolution.v1",
				"failure_class":  "source_not_found",
				"failure_detail": "streaming is disabled for this camera",
			},
		},
	}

	for name, tc := range cases {
		payload, err := tc.resolution.Serialize()
		if err != nil {
			t.Fatalf("%s: serialize: %v", name, err)
		}
		var got map[string]any
		if err := json.Unmarshal(payload, &got); err != nil {
			t.Fatalf("%s: decode: %v", name, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: document = %v, want %v", name, got, tc.want)
		}
	}

	for name, resolution := range map[string]CameraSourceResolution{
		"neither": {},
		"both":    {URL: "rtsp://192.0.2.20/stream", FailureClass: CameraSourceFailureNotFound},
	} {
		if _, err := resolution.Serialize(); err == nil {
			t.Errorf("%s: serialize succeeded, want an error", name)
		}
	}
}

func TestParseOmitCameraStreamAddresses(t *testing.T) {
	cases := map[string]bool{
		`{"_serviceradar_camera_stream_addresses":"omit","host":"192.0.2.30"}`: true,
		`{"_serviceradar_camera_stream_addresses":"report"}`:                   false,
		`{"host":"192.0.2.30"}`: false,
		``:                      false,
	}
	for config, want := range cases {
		got, err := ParseOmitCameraStreamAddresses([]byte(config))
		if err != nil || got != want {
			t.Errorf("config %q: omit=%v err=%v, want %v", config, got, err, want)
		}
	}

	for _, value := range []string{`true`, `null`, `1`, `{}`, `["omit"]`} {
		config := `{"_serviceradar_camera_stream_addresses":` + value + `}`
		if _, err := ParseOmitCameraStreamAddresses([]byte(config)); err == nil {
			t.Errorf("non-string value %s must be an error", value)
		}
	}
}
