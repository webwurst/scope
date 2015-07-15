package sniff

import (
	"io"
	"log"
	"sync/atomic"
	"time"

	"github.com/weaveworks/scope/report"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// Sniffer is a packet-sniffing reporter.
type Sniffer struct {
	hostID  string
	reports chan report.Report
	parser  *gopacket.DecodingLayerParser
	decoded []gopacket.LayerType
	eth     layers.Ethernet
	ip4     layers.IPv4
	ip6     layers.IPv6
	tcp     layers.TCP
	udp     layers.UDP
	icmp4   layers.ICMPv4
	icmp6   layers.ICMPv6
}

// New returns a new sniffing reporter that samples traffic by turning its
// packet capture facilities on and off.
func New(hostID string, src gopacket.ZeroCopyPacketDataSource, on, off time.Duration) *Sniffer {
	s := &Sniffer{
		hostID:  hostID,
		reports: make(chan report.Report),
	}
	s.parser = gopacket.NewDecodingLayerParser(
		layers.LayerTypeEthernet,
		&s.eth, &s.ip4, &s.ip6, &s.tcp, &s.udp, &s.icmp4, &s.icmp6,
	)
	go s.loop(src, on, off)
	return s
}

// Report implements the Reporter interface.
func (s *Sniffer) Report() (report.Report, error) {
	return <-s.reports, nil
}

func (s *Sniffer) loop(src gopacket.ZeroCopyPacketDataSource, on, off time.Duration) {
	var (
		process = uint64(1)               // initially enabled
		total   = uint64(0)               // total packets seen
		count   = uint64(0)               // count of packets captured
		packets = make(chan packet)       // decoded packets
		rpt     = report.MakeReport()     // the report we build
		turnOn  = (<-chan time.Time)(nil) // signal to start capture (initially enabled)
		turnOff = time.After(on)          // signal to stop capture
	)

	go s.read(src, packets, &process, &total, &count)

	for {
		select {
		case p := <-packets:
			s.merge(p, rpt)

		case <-turnOn:
			atomic.StoreUint64(&process, 1) // enable packet capture
			turnOn = nil                    // disable the on switch
			turnOff = time.After(on)        // enable the off switch

		case <-turnOff:
			atomic.StoreUint64(&process, 0) // disable packet capture
			turnOn = time.After(off)        // enable the on switch
			turnOff = nil                   // disable the off switch

		case s.reports <- rpt:
			rpt = report.MakeReport()
		}
	}
}

// An intermediate, decoded form of a packet, containing the information that
// the Scope data model cares about. Designed to decouple the packet data
// source loop, which should be as fast as possible, and the process of
// merging the packet information to a report, which may take some time and
// allocations.
type packet struct {
	srcIP, dstIP       string
	srcPort, dstPort   string
	network, transport int // byte counts
}

func (s *Sniffer) read(src gopacket.ZeroCopyPacketDataSource, dst chan packet, process, total, count *uint64) {
	var (
		p    packet
		data []byte
		err  error
	)
	for {
		data, _, err = src.ZeroCopyReadPacketData()
		if err == io.EOF {
			return // done
		}
		if err != nil {
			log.Printf("sniffer: read: %v", err)
			continue
		}
		atomic.AddUint64(total, 1)
		if atomic.LoadUint64(process) == 0 {
			continue
		}

		s.parser.DecodeLayers(data, &s.decoded)
		for _, t := range s.decoded {
			switch t {
			case layers.LayerTypeEthernet:
				//

			case layers.LayerTypeICMPv4:
				p.network += len(s.icmp4.Payload)

			case layers.LayerTypeICMPv6:
				p.network += len(s.icmp6.Payload)

			case layers.LayerTypeIPv6:
				p.srcIP = s.ip6.SrcIP.String()
				p.dstIP = s.ip6.DstIP.String()
				p.network += len(s.ip6.Payload)

			case layers.LayerTypeIPv4:
				p.srcIP = s.ip4.SrcIP.String()
				p.dstIP = s.ip4.DstIP.String()
				p.network += len(s.ip4.Payload)

			case layers.LayerTypeTCP:
				p.srcPort = s.tcp.SrcPort.String()
				p.dstPort = s.tcp.DstPort.String()
				p.transport += len(s.tcp.Payload)

			case layers.LayerTypeUDP:
				p.srcPort = s.udp.SrcPort.String()
				p.dstPort = s.udp.DstPort.String()
				p.transport += len(s.udp.Payload)
			}
		}

		atomic.AddUint64(count, 1)
		dst <- p
	}
}

func (s *Sniffer) merge(p packet, rpt report.Report) {
	// With a src and dst IP, we can add to the address topology.
	if p.srcIP != "" && p.dstIP != "" {
		var (
			srcNodeID      = report.MakeAddressNodeID(s.hostID, p.srcIP)
			dstNodeID      = report.MakeAddressNodeID(s.hostID, p.dstIP)
			edgeID         = report.MakeEdgeID(srcNodeID, dstNodeID)
			srcAdjacencyID = report.MakeAdjacencyID(srcNodeID)
			dstAdjacencyID = report.MakeAdjacencyID(dstNodeID)
		)
		rpt.Address.NodeMetadatas[srcNodeID] = report.NodeMetadata{} // TODO can we add something here?
		rpt.Address.NodeMetadatas[dstNodeID] = report.NodeMetadata{} // TODO can we add something here?

		emd := rpt.Address.EdgeMetadatas[edgeID]
		emd.WithBytes = true
		emd.BytesEgress += uint(p.network) // TODO is this right? may need to play games with LocalNetworks...
		rpt.Address.EdgeMetadatas[edgeID] = emd

		rpt.Address.Adjacency[srcAdjacencyID] = rpt.Address.Adjacency[srcAdjacencyID].Add(dstAdjacencyID)
	}

	// With a src and dst IP and port, we can add to the endpoints.
	if p.srcIP != "" && p.dstIP != "" && p.srcPort != "" && p.dstPort != "" {
		var (
			srcNodeID      = report.MakeEndpointNodeID(s.hostID, p.srcIP, p.srcPort)
			dstNodeID      = report.MakeEndpointNodeID(s.hostID, p.dstIP, p.dstPort)
			edgeID         = report.MakeEdgeID(srcNodeID, dstNodeID)
			srcAdjacencyID = report.MakeAdjacencyID(srcNodeID)
			dstAdjacencyID = report.MakeAdjacencyID(dstNodeID)
		)
		rpt.Endpoint.NodeMetadatas[srcNodeID] = report.NodeMetadata{} // TODO can we add something here?
		rpt.Endpoint.NodeMetadatas[dstNodeID] = report.NodeMetadata{} // TODO can we add something here?

		emd := rpt.Endpoint.EdgeMetadatas[edgeID]
		emd.WithBytes = true
		emd.BytesEgress += uint(p.transport) // TODO is this right? may need to play games with LocalNetworks...
		rpt.Endpoint.EdgeMetadatas[edgeID] = emd

		rpt.Endpoint.Adjacency[srcAdjacencyID] = rpt.Endpoint.Adjacency[srcAdjacencyID].Add(dstAdjacencyID)
	}
}
