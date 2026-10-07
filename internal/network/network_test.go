package network

import "testing"

func TestGatewayIP(t *testing.T) {
	got := gatewayIP("default via 10.0.0.1 dev eth0 proto dhcp")
	if got != "10.0.0.1" {
		t.Fatalf("got %q", got)
	}
}
