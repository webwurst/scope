package sniff

import (
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

// Source describes a packet data source that can be terminated.
type Source interface {
	gopacket.ZeroCopyPacketDataSource
	Close()
}

// SourceFactory constructs a new source for one-time use.
type SourceFactory func() (Source, error)

const (
	snaplen = 65535
	promisc = true
	timeout = pcap.BlockForever
)

// NewSourceFactory returns a live packet data source via the passed device
// (interface).
func NewSourceFactory(device string) SourceFactory {
	return func() (Source, error) {
		return pcap.OpenLive(device, snaplen, promisc, timeout)
	}
}
