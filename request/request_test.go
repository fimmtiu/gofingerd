package request

import "testing"

func TestParseRequest_NoInput(t *testing.T) {
	r := ParseRequest("")
	if len(r.User) > 0 || len(r.Hosts) > 0 {
		t.Fail()
	}
}

func TestParseRequest_StripW(t *testing.T) {
	r := ParseRequest("/W user@host")
	if r.User != "user" || len(r.Hosts) != 1 || r.Hosts[0] != "host" {
		t.Fail()
	}
}

func TestParseRequest_StripSpace(t *testing.T) {
	r := ParseRequest("\f    user@host \t  ")
	if r.User != "user" || len(r.Hosts) != 1 || r.Hosts[0] != "host" {
		t.Fail()
	}
}

func TestParseRequest_TwoHosts(t *testing.T) {
	r := ParseRequest("user@host1@host2")
	if r.User != "user" || len(r.Hosts) != 2 || r.Hosts[0] != "host1" || r.Hosts[1] != "host2" {
		t.Fail()
	}
}

// Should panic if the request isn't a forwarding request.
func TestNextForwardingRequest_NoHosts(t *testing.T) {
	defer func() { recover() }()
	r := ParseRequest("user")
	r.NextForwardingRequest()
	t.Fail()
}

func TestNextForwardingRequest_OneHost(t *testing.T) {
	r := ParseRequest("user@host1")
	host, fwd := r.NextForwardingRequest()
	if host != "host1" || fwd != "user\r\n" {
		t.Fail()
	}
}

func TestNextForwardingRequest_TwoHosts(t *testing.T) {
	r := ParseRequest("user@host1@host2")
	host, fwd := r.NextForwardingRequest()
	if host != "host2" || fwd != "user@host1\r\n" {
		t.Fail()
	}
}
