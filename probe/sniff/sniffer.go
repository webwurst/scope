package sniff

import (
	"io"
	"log"
	"time"

	"github.com/weaveworks/scope/report"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type sniffer struct {
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
	quit    chan struct{}
}

func newSniffer(hostID string, factory func() source, on, off time.Duration) *sniffer {
	s := &sniffer{
		hostID:  hostID,
		reports: make(chan report.Report),
		quit:    make(chan struct{}),
	}
	s.parser = gopacket.NewDecodingLayerParser(
		layers.LayerTypeEthernet,
		&s.eth, &s.ip4, &s.ip6, &s.tcp, &s.udp, &s.icmp4, &s.icmp6,
	)
	go s.loop(factory, on, off)
	return s
}

func (s *sniffer) stop() {
	close(s.quit)
}

func (s *sniffer) loop(factory func() source, on, off time.Duration) {
	for {
		// Start a new data source, prepare a new report.
		var (
			source = factory()
			rpt    = report.MakeReport()
			done   = make(chan struct{})
		)

		// We need to shut it down after our interval.
		go func() {
			time.Sleep(on)
			source.Close()
		}()

		// Read all the packets.
		go s.read(source, rpt, done)

		// Finish, publish the report, wait for the next iteration.
		select {
		case <-done:
		case <-s.quit:
			return
		}

		s.reports <- rpt

		select {
		case <-time.After(off):
		case <-s.quit:
			return
		}
	}
}

func (s *sniffer) read(src gopacket.ZeroCopyPacketDataSource, rpt report.Report, done chan struct{}) {
	defer close(done)
	for {
		data, _, err := src.ZeroCopyReadPacketData()
		if err == io.EOF {
			return // done
		}
		if err != nil {
			log.Printf("sniffer: read: %v", err)
			continue
		}

		var (
			srcIP, dstIP       string
			srcPort, dstPort   string
			network, transport int
		)

		s.parser.DecodeLayers(data, &s.decoded)
		for _, t := range s.decoded {
			switch t {
			case layers.LayerTypeEthernet:
				//

			case layers.LayerTypeICMPv4:
				network += len(s.icmp4.Payload)

			case layers.LayerTypeICMPv6:
				network += len(s.icmp6.Payload)

			case layers.LayerTypeIPv6:
				srcIP = s.ip6.SrcIP.String()
				dstIP = s.ip6.DstIP.String()
				network += len(s.ip6.Payload)

			case layers.LayerTypeIPv4:
				srcIP = s.ip4.SrcIP.String()
				dstIP = s.ip4.DstIP.String()
				network += len(s.ip4.Payload)

			case layers.LayerTypeTCP:
				srcPort = s.tcp.SrcPort.String()
				dstPort = s.tcp.DstPort.String()
				transport += len(s.tcp.Payload)

			case layers.LayerTypeUDP:
				srcPort = s.udp.SrcPort.String()
				dstPort = s.udp.DstPort.String()
				transport += len(s.udp.Payload)
			}
		}

		// With a src and dst IP, we can add to the address topology.
		if srcIP != "" && dstIP != "" {
			var (
				srcNodeID      = report.MakeAddressNodeID(s.hostID, srcIP)
				dstNodeID      = report.MakeAddressNodeID(s.hostID, dstIP)
				edgeID         = report.MakeEdgeID(srcNodeID, dstNodeID)
				srcAdjacencyID = report.MakeAdjacencyID(srcNodeID)
				dstAdjacencyID = report.MakeAdjacencyID(dstNodeID)
			)
			rpt.Address.NodeMetadatas[srcNodeID] = report.NodeMetadata{} // TODO can we add something here?
			rpt.Address.NodeMetadatas[dstNodeID] = report.NodeMetadata{} // TODO can we add something here?

			emd := rpt.Address.EdgeMetadatas[edgeID]
			emd.WithBytes = true
			emd.BytesEgress += uint(network) // TODO is this right? may need to play games with LocalNetworks...
			rpt.Address.EdgeMetadatas[edgeID] = emd

			rpt.Address.Adjacency[srcAdjacencyID] = rpt.Address.Adjacency[srcAdjacencyID].Add(dstAdjacencyID)
		}

		// With a src and dst IP and port, we can add to the endpoints.
		if srcIP != "" && dstIP != "" && srcPort != "" && dstPort != "" {
			var (
				srcNodeID      = report.MakeEndpointNodeID(s.hostID, srcIP, srcPort)
				dstNodeID      = report.MakeEndpointNodeID(s.hostID, dstIP, dstPort)
				edgeID         = report.MakeEdgeID(srcNodeID, dstNodeID)
				srcAdjacencyID = report.MakeAdjacencyID(srcNodeID)
				dstAdjacencyID = report.MakeAdjacencyID(dstNodeID)
			)
			rpt.Endpoint.NodeMetadatas[srcNodeID] = report.NodeMetadata{} // TODO can we add something here?
			rpt.Endpoint.NodeMetadatas[dstNodeID] = report.NodeMetadata{} // TODO can we add something here?

			emd := rpt.Endpoint.EdgeMetadatas[edgeID]
			emd.WithBytes = true
			emd.BytesEgress += uint(transport) // TODO is this right? may need to play games with LocalNetworks...
			rpt.Endpoint.EdgeMetadatas[edgeID] = emd

			rpt.Endpoint.Adjacency[srcAdjacencyID] = rpt.Endpoint.Adjacency[srcAdjacencyID].Add(dstAdjacencyID)
		}
	}
}

type source interface {
	gopacket.ZeroCopyPacketDataSource
	Close()
}
