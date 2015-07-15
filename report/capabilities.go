package report

// Capabilities describe the capability tags within the NodeMetadata
type Capabilities map[string]Capability

// A Capability basically describes an RPC
type Capability struct {
	ID    string
	Human string
	Args  []Arg
}

type Arg struct {
	Name  string
	Human string
	Type  ArgType
}

type ArgType int

const (
	Duration ArgType = iota
)

func (cs Capabilities) Merge(other Capabilities) {
	for k, v := range other {
		cs[k] = v
	}
}
