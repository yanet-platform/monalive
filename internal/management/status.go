package management

import (
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/yanet-platform/monalive/internal/core/checker"
	"github.com/yanet-platform/monalive/internal/core/real"
	"github.com/yanet-platform/monalive/internal/core/service"
	monalivepb "github.com/yanet-platform/monalive/monalivepb/v1"
)

// ServiceStatuses converts core snapshots to the management wire representation.
func ServiceStatuses(snapshots []service.Status) []*monalivepb.ServiceStatus {
	statuses := make([]*monalivepb.ServiceStatus, 0, len(snapshots))
	for _, snapshot := range snapshots {
		group := snapshot.AnnounceGroup
		reals := make([]*monalivepb.RealStatus, 0, len(snapshot.Reals))
		for _, real := range snapshot.Reals {
			reals = append(reals, realStatus(real))
		}

		statuses = append(statuses, &monalivepb.ServiceStatus{
			Vip:                    snapshot.VIP.String(),
			Port:                   snapshot.Port.ProtoMarshaller(),
			Protocol:               snapshot.Protocol,
			LvsMethod:              snapshot.LVSMethod,
			QuorumState:            numericBool(snapshot.QuorumState),
			AliveWeight:            snapshot.AliveWeight.Uint32(),
			AliveCount:             uint32(snapshot.AliveCount),
			Transitions:            uint32(snapshot.Transitions),
			Fwmark:                 uint32(snapshot.FWMark),
			AnnounceGroup:          &group,
			Ipv4OuterSourceNetwork: snapshot.IPv4OuterSourceNetwork,
			Ipv6OuterSourceNetwork: snapshot.IPv6OuterSourceNetwork,
			Rs:                     reals,
			QuorumUp:               snapshot.QuorumUp,
		})
	}

	return statuses
}

func realStatus(snapshot real.Status) *monalivepb.RealStatus {
	checkers := make([]*monalivepb.CheckerStatus, 0, len(snapshot.Checkers))
	for _, checker := range snapshot.Checkers {
		checkers = append(checkers, checkerStatus(checker))
	}

	return &monalivepb.RealStatus{
		Ip:          snapshot.IP.String(),
		Port:        snapshot.Port.ProtoMarshaller(),
		Alive:       numericBool(snapshot.Alive),
		Weight:      snapshot.Weight.Uint32(),
		Transitions: uint32(snapshot.Transitions),
		Checkers:    checkers,
	}
}

func checkerStatus(snapshot checker.Status) *monalivepb.CheckerStatus {
	var virtualhost *string
	if snapshot.Virtualhost != nil {
		value := *snapshot.Virtualhost
		virtualhost = &value
	}

	return &monalivepb.CheckerStatus{
		Type:                snapshot.Type,
		ConnectIp:           snapshot.ConnectIP.String(),
		ConnectPort:         snapshot.ConnectPort.ProtoMarshaller(),
		BindIp:              snapshot.BindIP.String(),
		ConnectTimeout:      durationpb.New(snapshot.ConnectTimeout),
		CheckTimeout:        durationpb.New(snapshot.CheckTimeout),
		Fwmark:              uint32(snapshot.FWMark),
		Path:                snapshot.Path,
		StatusCode:          int32(snapshot.StatusCode),
		Digest:              snapshot.Digest,
		Virtualhost:         virtualhost,
		DynamicWeight:       snapshot.DynamicWeight,
		DynamicWeightHeader: snapshot.DynamicWeightHeader,
		DynamicWeightCoeff:  uint32(snapshot.DynamicWeightCoeff),
		DelayLoop:           durationpb.New(snapshot.DelayLoop),
		Retries:             uint32(snapshot.Retries),
		RetryDelay:          durationpb.New(snapshot.RetryDelay),
		Alive:               numericBool(snapshot.Alive),
		FailedAttempts:      uint32(snapshot.FailedAttempts),
		LastCheckTs:         timestamppb.New(snapshot.LastCheckTS),
	}
}

func numericBool(value bool) uint32 {
	if value {
		return 1
	}

	return 0
}
