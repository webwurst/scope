package endpoint

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/weaveworks/scope/common/exec"
)

// Constants exported for testing
const (
	modules          = "/proc/modules"
	conntrackModule  = "nf_conntrack"
	XMLHeader        = "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n"
	ConntrackOpenTag = "<conntrack>\n"
	TimeWait         = "TIME_WAIT"
	TCP              = "tcp"
	New              = "new"
	Update           = "update"
	Destroy          = "destroy"
)

// Layer3 - these structs are for the parsed conntrack output
type Layer3 struct {
	XMLName xml.Name `xml:"layer3"`
	SrcIP   string   `xml:"src"`
	DstIP   string   `xml:"dst"`
}

// Layer4 - these structs are for the parsed conntrack output
type Layer4 struct {
	XMLName xml.Name `xml:"layer4"`
	SrcPort int      `xml:"sport"`
	DstPort int      `xml:"dport"`
	Proto   string   `xml:"protoname,attr"`
}

// Meta - these structs are for the parsed conntrack output
type Meta struct {
	XMLName   xml.Name `xml:"meta"`
	Direction string   `xml:"direction,attr"`
	Layer3    Layer3   `xml:"layer3"`
	Layer4    Layer4   `xml:"layer4"`
	ID        int64    `xml:"id"`
	State     string   `xml:"state"`
}

// Flow - these structs are for the parsed conntrack output
type Flow struct {
	XMLName xml.Name `xml:"flow"`
	Metas   []Meta   `xml:"meta"`
	Type    string   `xml:"type,attr"`

	Original, Reply, Independent *Meta `xml:"-"`
}

type conntrack struct {
	XMLName xml.Name `xml:"conntrack"`
	Flows   []Flow   `xml:"flow"`
}

// Conntracker is something that tracks connections.
type Conntracker interface {
	WalkFlows(f func(Flow))
	Stop()
}

// Conntracker uses the conntrack command to track network connections
type conntracker struct {
	sync.Mutex
	cmd           exec.Cmd
	activeFlows   map[int64]Flow // active flows in state != TIME_WAIT
	bufferedFlows []Flow         // flows coming out of activeFlows spend 1 walk cycle here
	existingConns bool
}

// NewConntracker creates and starts a new Conntracter
func NewConntracker(existingConns bool, args ...string) (Conntracker, error) {
	if !ConntrackModulePresent() {
		return nil, fmt.Errorf("No conntrack module")
	}
	result := &conntracker{
		activeFlows:   map[int64]Flow{},
		existingConns: existingConns,
	}
	go result.run(args...)
	return result, nil
}

// ConntrackModulePresent returns true if the kernel has the conntrack module
// present.  It is made public for mocking.
var ConntrackModulePresent = func() bool {
	f, err := os.Open(modules)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, conntrackModule) {
			return true
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("conntrack error: %v", err)
	}

	log.Printf("conntrack: failed to find module %s", conntrackModule)
	return false
}

// NB this is not re-entrant!
func (c *conntracker) run(args ...string) {
	if c.existingConns {
		// Fork another conntrack, just to capture existing connections
		// for which we don't get events
		existingFlows, err := c.existingConnections(args...)
		if err != nil {
			log.Printf("conntrack existingConnections error: %v", err)
			return
		}
		for _, flow := range existingFlows {
			c.handleFlow(flow, true)
		}
	}

	args = append([]string{"-E", "-o", "xml", "-p", "tcp"}, args...)
	cmd := exec.Command("conntrack", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("conntrack error: %v", err)
		return
	}
	if err := cmd.Start(); err != nil {
		log.Printf("conntrack error: %v", err)
		return
	}

	c.Lock()
	c.cmd = cmd
	c.Unlock()
	defer func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("conntrack error: %v", err)
		}
	}()

	// Swallow the first two lines
	reader := bufio.NewReader(stdout)
	if line, err := reader.ReadString('\n'); err != nil {
		log.Printf("conntrack error: %v", err)
		return
	} else if line != XMLHeader {
		log.Printf("conntrack invalid output: '%s'", line)
		return
	}
	if line, err := reader.ReadString('\n'); err != nil {
		log.Printf("conntrack error: %v", err)
		return
	} else if line != ConntrackOpenTag {
		log.Printf("conntrack invalid output: '%s'", line)
		return
	}

	defer log.Printf("contrack exiting")

	// Now loop on the output stream
	decoder := xml.NewDecoder(reader)
	for {
		var f Flow
		if err := decoder.Decode(&f); err != nil {
			log.Printf("conntrack error: %v", err)
			return
		}
		c.handleFlow(f, false)
	}
}

func (c *conntracker) existingConnections(args ...string) ([]Flow, error) {
	args = append([]string{"-L", "-o", "xml", "-p", "tcp"}, args...)
	cmd := exec.Command("conntrack", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return []Flow{}, err
	}
	if err := cmd.Start(); err != nil {
		return []Flow{}, err
	}
	defer func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("conntrack existingConnections exit error: %v", err)
		}
	}()
	var result conntrack
	if err := xml.NewDecoder(stdout).Decode(&result); err == io.EOF {
		return []Flow{}, nil
	} else if err != nil {
		return []Flow{}, err
	}
	return result.Flows, nil
}

// Stop stop stop
func (c *conntracker) Stop() {
	c.Lock()
	defer c.Unlock()
	if c.cmd == nil {
		return
	}

	if p := c.cmd.Process(); p != nil {
		p.Kill()
	}
}

func (c *conntracker) handleFlow(f Flow, forceAdd bool) {
	// A flow consists of 3 'metas' - the 'original' 4 tuple (as seen by this
	// host) and the 'reply' 4 tuple, which is what it has been rewritten to.
	// This code finds those metas, which are identified by a Direction
	// attribute.
	for i := range f.Metas {
		meta := &f.Metas[i]
		switch meta.Direction {
		case "original":
			f.Original = meta
		case "reply":
			f.Reply = meta
		case "independent":
			f.Independent = meta
		}
	}

	// For not, I'm only interested in tcp connections - there is too much udp
	// traffic going on (every container talking to weave dns, for example) to
	// render nicely. TODO: revisit this.
	if f.Original.Layer4.Proto != TCP {
		return
	}

	c.Lock()
	defer c.Unlock()

	switch {
	case forceAdd || f.Type == New || f.Type == Update:
		if f.Independent.State != TimeWait {
			c.activeFlows[f.Independent.ID] = f
		} else if _, ok := c.activeFlows[f.Independent.ID]; ok {
			delete(c.activeFlows, f.Independent.ID)
			c.bufferedFlows = append(c.bufferedFlows, f)
		}
	case f.Type == Destroy:
		if _, ok := c.activeFlows[f.Independent.ID]; ok {
			delete(c.activeFlows, f.Independent.ID)
			c.bufferedFlows = append(c.bufferedFlows, f)
		}
	}
}

// WalkFlows calls f with all active flows and flows that have come and gone
// since the last call to WalkFlows
func (c *conntracker) WalkFlows(f func(Flow)) {
	c.Lock()
	defer c.Unlock()
	for _, flow := range c.activeFlows {
		f(flow)
	}
	for _, flow := range c.bufferedFlows {
		f(flow)
	}
	c.bufferedFlows = c.bufferedFlows[:0]
}
