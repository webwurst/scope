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

// New returns a new sniffing Reporter that samples at the given rate over the
// default time quantum (window). For example, if sampleRate is 0.01, and
// quantum is 5s, then the sniffer will be on and sniffing traffic for 0.01 *
// 5s = 50ms, and then off for 5s - 50ms = 4950ms.
func New(hostID string, factory func() source, sampleRate float64) *Reporter {
	r := &Reporter{
		reports: make(chan report.Report),
		quit:    make(chan struct{}),
	}
	var (
		on  = time.Duration(sampleRate * float64(quantum))
		off = quantum - on
	)
	go r.loop(hostID, factory, on, off)
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

var quantum = 5 * time.Second

func (r *Reporter) loop(hostID string, factory func() source, on, off time.Duration) {
	var (
		s   = newSniffer(hostID, factory, on, off)
		rpt = report.MakeReport()
	)
	defer s.stop()
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

func flipflop(quantum time.Duration, rate float64, sleep func(time.Duration)) (on, off chan struct{}) {
	on, off = make(chan struct{}), make(chan struct{})
	go func() {
		var (
			onTime  = time.Duration(float64(quantum) * rate)
			offTime = quantum - onTime
		)
		for {
			on <- struct{}{}
			sleep(onTime)
			off <- struct{}{}
			sleep(offTime)
		}
	}()
	return on, off
}
