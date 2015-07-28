package render_test

import (
	"reflect"
	"testing"

	"github.com/weaveworks/scope/probe/host"
	"github.com/weaveworks/scope/render"
	"github.com/weaveworks/scope/report"
	"github.com/weaveworks/scope/test"
)

func TestMakeDetailedNode(t *testing.T) {
	renderableNode := render.ContainerRenderer.Render(test.Report)[test.ServerContainerID]
	have := render.MakeDetailedNode(test.Report, renderableNode)
	want := render.DetailedNode{
		ID:         test.ServerContainerID,
		LabelMajor: "server",
		LabelMinor: test.ServerHostName,
		Pseudo:     false,
		Sections: map[string]render.Section{
			render.SectionHosts: render.Section{
				test.ServerHostNodeID: map[string]string{
					host.HostName:      test.ServerHostName,
					host.OS:            test.ServerHostOS,
					host.Load:          test.ServerHostLoad,
					host.Uptime:        test.ServerHostUptime,
					host.KernelVersion: test.ServerHostKernelVersion,
				},
			},
			render.SectionEndpoints: render.Section{
				report.MakeEdgeID(test.Server80NodeID, test.UnknownClient1NodeID): render.Section{
					render.ConnectionSrc:        test.ServerIP + ":" + test.ServerPort,
					render.ConnectionDst:        test.UnknownClient1IP + ":" + test.ClientPort54010,
					render.ConnectionPacketRate: "20",
					render.ConnectionByteRate:   "200",
				},
				report.MakeEdgeID(test.Server80NodeID, test.UnknownClient2NodeID): render.Section{
					render.ConnectionSrc:        test.ServerIP + ":" + test.ServerPort,
					render.ConnectionDst:        test.UnknownClient1IP + ":" + test.ClientPort54020,
					render.ConnectionPacketRate: "26",
					render.ConnectionByteRate:   "266",
				},
				report.MakeEdgeID(test.Server80NodeID, test.UnknownClient3NodeID): render.Section{
					render.ConnectionSrc:        test.ServerIP + ":" + test.ServerPort,
					render.ConnectionDst:        test.UnknownClient3IP + ":" + test.ClientPort54020,
					render.ConnectionPacketRate: "33",
					render.ConnectionByteRate:   "333",
				},
				report.MakeEdgeID(test.Server80NodeID, test.Client54001NodeID): render.Section{
					render.ConnectionSrc:        test.ServerIP + ":" + test.ServerPort,
					render.ConnectionDst:        test.ClientIP + ":" + test.ClientPort54001,
					render.ConnectionPacketRate: "6",
					render.ConnectionByteRate:   "66",
				},
				report.MakeEdgeID(test.Server80NodeID, test.Client54002NodeID): render.Section{
					render.ConnectionSrc:        test.ServerIP + ":" + test.ServerPort,
					render.ConnectionDst:        test.ClientIP + ":" + test.ClientPort54002,
					render.ConnectionPacketRate: "13",
					render.ConnectionByteRate:   "133",
				},
				report.MakeEdgeID(test.Server80NodeID, test.RandomClientNodeID): render.Section{
					render.ConnectionSrc: test.ServerIP + ":" + test.ServerPort,
					render.ConnectionDst: test.RandomClientIP + ":" + test.ClientPort12345,
				},
			},
		},
	}
	if !reflect.DeepEqual(want, have) {
		t.Error(test.Diff(want, have))
	}
}
