package render

import (
	"github.com/weaveworks/scope/report"
)

// TODO deprecate and remove

// AggregateMetadata is an intermediate type that should be removed.
type AggregateMetadata struct{ report.EdgeMetadata }

// Merge merges an aggregate metadata into this one.
func (m *AggregateMetadata) Merge(other AggregateMetadata) {
	m.EdgeMetadata.Merge(other.EdgeMetadata)
}
