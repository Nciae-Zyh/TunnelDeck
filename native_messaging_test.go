package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNativeMessageFraming(t *testing.T) {
	var buffer bytes.Buffer
	writer := nativeWriter{output: &buffer}
	want := nativeResponse{ID: "request-1", OK: true, Result: map[string]string{"status": "ok"}}
	if err := writer.write(want); err != nil {
		t.Fatal(err)
	}
	payload, err := readNativeMessage(&buffer)
	if err != nil {
		t.Fatal(err)
	}
	var got nativeResponse
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != want.ID || !got.OK {
		t.Fatalf("unexpected response: %#v", got)
	}
}

func TestNativeMessageRejectsOversizedFrame(t *testing.T) {
	frame := []byte{1, 0, 16, 0}
	if _, err := readNativeMessage(bytes.NewReader(frame)); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected size error, got %v", err)
	}
}

func TestNativeMessagingLaunchDetection(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{args: []string{"TunnelDeck", "native-host"}, want: true},
		{args: []string{"TunnelDeck", "chrome-extension://abcdefghijklmnopabcdefghijklmnop/"}, want: true},
		{args: []string{"TunnelDeckNativeHost"}, want: true},
		{args: []string{"TunnelDeck"}, want: false},
	}
	for _, test := range tests {
		if got := isNativeMessagingLaunch(test.args); got != test.want {
			t.Fatalf("isNativeMessagingLaunch(%q) = %v, want %v", test.args, got, test.want)
		}
	}
}

func TestNativeHostPing(t *testing.T) {
	host := nativeHost{}
	result, responseErr := host.handle(nativeRequest{ID: "ping-1", Method: "ping"})
	if responseErr != nil {
		t.Fatalf("ping failed: %#v", responseErr)
	}
	values, ok := result.(map[string]any)
	if !ok || values["host"] != nativeHostName || values["protocolVersion"] != nativeProtocolVersion {
		t.Fatalf("unexpected ping result: %#v", result)
	}
}

func TestNativeMessagingHostPingRoundTrip(t *testing.T) {
	var input bytes.Buffer
	inputWriter := nativeWriter{output: &input}
	if err := inputWriter.write(nativeRequest{ID: "ping-round-trip", Method: "ping"}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runNativeMessagingHostWithApp(&App{}, &input, &output); err != nil {
		t.Fatal(err)
	}
	payload, err := readNativeMessage(&output)
	if err != nil {
		t.Fatal(err)
	}
	var response nativeResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatal(err)
	}
	if response.ID != "ping-round-trip" || !response.OK {
		t.Fatalf("unexpected round-trip response: %#v", response)
	}
}
