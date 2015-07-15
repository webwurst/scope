package sniff

import (
	"time"

	"github.com/weaveworks/scope/report"
)

// Reporter generates reports containing the endpoint and address topologies.
type Reporter struct {
	reports chan report.Report
	quit    chan struct{}
}

var quantum = 5 * time.Second

// NewReporter returns a new sniffing Reporter that samples at the given rate
// over the default time quantum (window). For example, if sampleRate is 0.01,
// and quantum is 5s, then the sniffer will be on and sniffing traffic for
// 0.01 * 5s = 50ms, and then off for 5s - 50ms = 4950ms.
func NewReporter(hostID string, factory SourceFactory, sampleRate float64) *Reporter {
	r := &Reporter{
		reports: make(chan report.Report),
		quit:    make(chan struct{}),
	}
	var (
		on  = time.Duration(sampleRate * float64(quantum))
		off = quantum - on
	)
	go r.loop(newSniffer(hostID, factory, on, off))
	return r
}

// Report implements Reporter.
func (r *Reporter) Report() (report.Report, error) {
	return <-r.reports, nil
}

// Stop terminates the reporter and underlying sniffer.
func (r *Reporter) Stop() {
	close(r.quit)
}

func (r *Reporter) loop(s *sniffer) {
	defer s.stop()
	rpt := report.MakeReport()
	for {
		select {
		case rpt0 := <-s.reports:
			rpt.Merge(rpt0)
		case r.reports <- rpt:
			rpt = report.MakeReport()
		case <-r.quit:
			return
		}
	}
}
