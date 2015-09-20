package request

import "strings"

type Request struct {
	User  string
	Hosts []string
}

// The RFC permits us to ignore the /W option. (2.5.4)
func ParseRequest(input string) Request {
	input = strings.ToLower(input)
	input = strings.TrimPrefix(input, "/w")
	input = strings.TrimSpace(input)
	parts := strings.Split(input, "@")
	return Request{parts[0], parts[1:]}
}

func (r *Request) NextForwardingRequest() (host string, forwarded_request string) {
	if len(r.Hosts) == 0 {
		panic("Nowhere to forward to! What are you doing?")
	}

	forwarded_request = r.User
	if len(r.Hosts) > 1 {
		further_hosts := strings.Join(r.Hosts[0:len(r.Hosts)-1], "@")
		forwarded_request = forwarded_request + "@" + further_hosts
	}
	return r.Hosts[len(r.Hosts)-1], forwarded_request + "\r\n"
}
