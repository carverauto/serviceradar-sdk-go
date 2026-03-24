package sdk

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestParseRTSPEndpoint(t *testing.T) {
	endpoint, err := ParseRTSPEndpoint("rtsp://root:secret@10.0.0.5:8554/axis-media/media.amp?stream=1", "", "")
	if err != nil {
		t.Fatalf("ParseRTSPEndpoint returned error: %v", err)
	}

	if endpoint.Host != "10.0.0.5" {
		t.Fatalf("unexpected host: %s", endpoint.Host)
	}
	if endpoint.Port != 8554 {
		t.Fatalf("unexpected port: %d", endpoint.Port)
	}
	if endpoint.Username != "root" || endpoint.Password != "secret" {
		t.Fatalf("unexpected credentials: %s/%s", endpoint.Username, endpoint.Password)
	}
	if endpoint.RequestURI != "/axis-media/media.amp?stream=1" {
		t.Fatalf("unexpected request uri: %s", endpoint.RequestURI)
	}
}

func TestBuildRTSPAuthorization(t *testing.T) {
	got := BuildRTSPAuthorization(RTSPEndpoint{Username: "root", Password: "secret"}, "DESCRIBE", "/axis-media/media.amp", nil)
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("root:secret"))
	if got != want {
		t.Fatalf("unexpected auth header: %s", got)
	}
}

func TestBuildRTSPAuthorizationDigest(t *testing.T) {
	endpoint := RTSPEndpoint{Username: "root", Password: "secret"}
	auth := &RTSPAuthState{
		Scheme:    "digest",
		Realm:     "AXIS",
		Nonce:     "abcdef",
		Opaque:    "opaque-token",
		Algorithm: "MD5",
		QOP:       "auth",
		CNonce:    "serviceradar",
	}

	got := BuildRTSPAuthorization(endpoint, "DESCRIBE", "/axis-media/media.amp", auth)
	if !strings.HasPrefix(got, "Digest ") {
		t.Fatalf("expected digest auth header, got %s", got)
	}
	if !strings.Contains(got, `username="root"`) || !strings.Contains(got, `realm="AXIS"`) {
		t.Fatalf("unexpected digest header: %s", got)
	}
	if !strings.Contains(got, "qop=auth") || !strings.Contains(got, "nc=00000001") {
		t.Fatalf("expected qop and nc fields, got %s", got)
	}
}

func TestParseRTSPAuthenticateHeaderDigest(t *testing.T) {
	auth, err := ParseRTSPAuthenticateHeader(`Digest realm="AXIS", nonce="abcdef", opaque="opaque-token", qop="auth,auth-int", algorithm=MD5`)
	if err != nil {
		t.Fatalf("ParseRTSPAuthenticateHeader returned error: %v", err)
	}
	if auth.Scheme != "digest" {
		t.Fatalf("unexpected scheme: %s", auth.Scheme)
	}
	if auth.Realm != "AXIS" || auth.Nonce != "abcdef" {
		t.Fatalf("unexpected digest auth values: %#v", auth)
	}
	if auth.QOP != "auth" {
		t.Fatalf("expected qop auth, got %s", auth.QOP)
	}
}

func TestParseRTSPResponse(t *testing.T) {
	raw := []byte("RTSP/1.0 200 OK\r\nCSeq: 2\r\nContent-Length: 17\r\nSession: 12345;timeout=60\r\n\r\nv=0\r\nm=video 0\r\n")
	resp, err := ParseRTSPResponse(raw)
	if err != nil {
		t.Fatalf("ParseRTSPResponse returned error: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	if resp.Headers["session"] != "12345;timeout=60" {
		t.Fatalf("unexpected session header: %s", resp.Headers["session"])
	}
	if resp.ContentLength != 17 {
		t.Fatalf("unexpected content length: %d", resp.ContentLength)
	}
}

func TestBuildRTSPRequestWithDigestAuthorization(t *testing.T) {
	req := BuildRTSPRequest(
		RTSPEndpoint{Username: "root", Password: "secret"},
		"SETUP",
		"/axis-media/media.amp/trackID=1",
		3,
		"session-1",
		&RTSPAuthState{
			Scheme:    "digest",
			Realm:     "AXIS",
			Nonce:     "abcdef",
			Algorithm: "MD5",
			QOP:       "auth",
			CNonce:    "serviceradar",
		},
		map[string]string{"Transport": "RTP/AVP/TCP;unicast;interleaved=0-1"},
	)

	if !strings.Contains(req, "Authorization: Digest ") {
		t.Fatalf("expected digest authorization header in request: %s", req)
	}
	if !strings.Contains(req, "Session: session-1") {
		t.Fatalf("expected session header in request: %s", req)
	}
}

func TestParseH264TrackFromSDP(t *testing.T) {
	endpoint := RTSPEndpoint{
		BaseURL:    "rtsp://10.0.0.5",
		RequestURI: "/axis-media/media.amp",
	}
	sdp := []byte("v=0\nm=video 0 RTP/AVP 96\na=rtpmap:96 H264/90000\na=control:trackID=1\n")

	track, err := ParseH264TrackFromSDP(endpoint, sdp)
	if err != nil {
		t.Fatalf("ParseH264TrackFromSDP returned error: %v", err)
	}
	if track.PayloadType != 96 {
		t.Fatalf("unexpected payload type: %d", track.PayloadType)
	}
	if track.ControlURL != "rtsp://10.0.0.5/axis-media/media.amp/trackID=1" {
		t.Fatalf("unexpected control url: %s", track.ControlURL)
	}
}

func TestParseInterleavedFrame(t *testing.T) {
	frame, err := ParseInterleavedFrame([]byte{'$', 0x00, 0x00, 0x04, 0xDE, 0xAD, 0xBE, 0xEF})
	if err != nil {
		t.Fatalf("ParseInterleavedFrame returned error: %v", err)
	}
	if frame.Channel != 0 {
		t.Fatalf("unexpected channel: %d", frame.Channel)
	}
	if string(frame.Payload) != string([]byte{0xDE, 0xAD, 0xBE, 0xEF}) {
		t.Fatalf("unexpected payload: %#v", frame.Payload)
	}
}

func TestParseRTPPacket(t *testing.T) {
	packet := []byte{
		0x80, 0xE0, 0x00, 0x02,
		0x00, 0x00, 0x03, 0xE8,
		0x12, 0x34, 0x56, 0x78,
		0x65, 0x88, 0x84,
	}

	payload, marker, timestamp, err := ParseRTPPacket(packet)
	if err != nil {
		t.Fatalf("ParseRTPPacket returned error: %v", err)
	}
	if !marker {
		t.Fatal("expected marker bit")
	}
	if timestamp != 1000 {
		t.Fatalf("unexpected timestamp: %d", timestamp)
	}
	if len(payload) != 3 || payload[0] != 0x65 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestH264DepacketizerSingleNAL(t *testing.T) {
	depacketizer := &RTSPH264Depacketizer{}

	accessUnit, keyframe, complete, err := depacketizer.Push([]byte{0x65, 0x88, 0x84}, true, 1000)
	if err != nil {
		t.Fatalf("Push returned error: %v", err)
	}
	if !complete {
		t.Fatal("expected complete access unit")
	}
	if !keyframe {
		t.Fatal("expected keyframe")
	}
	if string(accessUnit) != string([]byte{0x00, 0x00, 0x00, 0x01, 0x65, 0x88, 0x84}) {
		t.Fatalf("unexpected access unit: %#v", accessUnit)
	}
}

func TestH264DepacketizerFUA(t *testing.T) {
	depacketizer := &RTSPH264Depacketizer{}

	start := []byte{0x7C, 0x85, 0xAA, 0xBB}
	middle := []byte{0x7C, 0x05, 0xCC}
	end := []byte{0x7C, 0x45, 0xDD, 0xEE}

	if _, _, complete, err := depacketizer.Push(start, false, 1000); err != nil || complete {
		t.Fatalf("unexpected start result: complete=%v err=%v", complete, err)
	}
	if _, _, complete, err := depacketizer.Push(middle, false, 1000); err != nil || complete {
		t.Fatalf("unexpected middle result: complete=%v err=%v", complete, err)
	}

	accessUnit, keyframe, complete, err := depacketizer.Push(end, true, 1000)
	if err != nil {
		t.Fatalf("unexpected end error: %v", err)
	}
	if !complete {
		t.Fatal("expected complete access unit")
	}
	if !keyframe {
		t.Fatal("expected keyframe")
	}

	want := []byte{0x00, 0x00, 0x00, 0x01, 0x65, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE}
	if string(accessUnit) != string(want) {
		t.Fatalf("unexpected FU-A access unit: %#v", accessUnit)
	}
}
