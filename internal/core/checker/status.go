package checker

import (
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	monalivepb "github.com/yanet-platform/monalive/gen/manager"
)

// Status returns [monalivepb.CheckerStatus] messages representing the status of
// the checker.
func (m *Checker) Status() *monalivepb.CheckerStatus {
	state := m.State()
	alive := uint32(0)
	if state.Alive {
		alive = 1
	}
	return &monalivepb.CheckerStatus{
		Type: m.config.Type.String(),

		ConnectIp:      m.config.ConnectIP.String(),
		ConnectPort:    m.config.ConnectPort.ProtoMarshaller(),
		BindIp:         m.config.BindIP.String(),
		ConnectTimeout: durationpb.New(m.config.GetConnectTimeout()),
		CheckTimeout:   durationpb.New(m.config.GetCheckTimeout()),
		Fwmark:         uint32(m.config.FWMark),

		Path:        m.config.Path,
		StatusCode:  int32(m.config.StatusCode),
		Digest:      m.config.Digest,
		Virtualhost: m.config.Virtualhost,

		DynamicWeight:       m.config.DynamicWeight,
		DynamicWeightHeader: m.config.DynamicWeightHeader,
		DynamicWeightCoeff:  uint32(m.config.DynamicWeightCoeff),

		DelayLoop:  durationpb.New(m.config.GetDelayLoop()),
		Retries:    uint32(m.config.GetRetries()),
		RetryDelay: durationpb.New(m.config.GetRetryDelay()),

		Alive:          alive,
		FailedAttempts: uint32(state.FailedAttempts),
		LastCheckTs:    timestamppb.New(state.Timestamp),
	}
}
