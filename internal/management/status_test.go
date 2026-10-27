package management_test

import (
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/yanet-platform/monalive/internal/core/checker"
	"github.com/yanet-platform/monalive/internal/core/real"
	"github.com/yanet-platform/monalive/internal/core/service"
	"github.com/yanet-platform/monalive/internal/management"
	monalivepb "github.com/yanet-platform/monalive/monalivepb/v1"
	"github.com/yanet-platform/monalive/pkg/optport"
)

func Test_ServiceStatuses_OptionalPresenceSurvivesWireRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name string
		port optport.Port
		host *string
	}{
		{name: "Omitted", port: optport.Omitted},
		{name: "ExplicitZeroAndEmpty", port: 0, host: proto.String("")},
		{name: "NonzeroAndNamed", port: 443, host: proto.String("health.example")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := service.Status{Port: tc.port, QuorumUp: "legacy-handler", Reals: []real.Status{{
				Port: tc.port, Checkers: []checker.Status{{ConnectPort: tc.port, Virtualhost: tc.host}},
			}}}

			mapped := management.ServiceStatuses([]service.Status{snapshot})[0]
			data, err := proto.Marshal(mapped)
			if err != nil {
				t.Fatal(err)
			}

			var got monalivepb.ServiceStatus
			if err := proto.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}

			for _, port := range []*uint32{got.Port, got.Rs[0].Port, got.Rs[0].Checkers[0].ConnectPort} {
				if tc.port < 0 {
					if port != nil {
						t.Fatal("omitted port acquired presence")
					}
				} else if port == nil || *port != uint32(tc.port) {
					t.Fatalf("explicit port %d lost presence or value", tc.port)
				}
			}

			host := got.Rs[0].Checkers[0].Virtualhost
			if (host == nil) != (tc.host == nil) || host != nil && *host != *tc.host {
				t.Fatal("virtualhost presence or value changed")
			}
			if got.AnnounceGroup == nil || *got.AnnounceGroup != "" || got.QuorumUp != snapshot.QuorumUp {
				t.Fatal("empty announce group presence or legacy quorum_up lost")
			}

			if tc.host != nil {
				*mapped.Rs[0].Checkers[0].Virtualhost = "changed"
				if *tc.host == "changed" {
					t.Fatal("protobuf aliases snapshot virtualhost")
				}
			}
		})
	}
}

func Test_ServiceStatuses_StateAndTimeEncoding(t *testing.T) {
	stamp := time.Date(2026, 10, 24, 12, 30, 45, 123456789, time.FixedZone("offset", 3600))
	for _, alive := range []bool{false, true} {
		snapshot := service.Status{QuorumState: alive, Reals: []real.Status{{Alive: alive, Checkers: []checker.Status{{
			Alive: alive, LastCheckTS: stamp,
			ConnectTimeout: 1500 * time.Millisecond, CheckTimeout: 2500 * time.Millisecond,
			DelayLoop: 3 * time.Second, RetryDelay: 250 * time.Millisecond,
		}}}}}

		got := management.ServiceStatuses([]service.Status{snapshot})[0]
		check := got.Rs[0].Checkers[0]

		want := uint32(0)
		if alive {
			want = 1
		}
		if got.QuorumState != want || got.Rs[0].Alive != want || check.Alive != want {
			t.Fatalf("alive=%v must encode as %d at every level", alive, want)
		}

		if !check.LastCheckTs.AsTime().Equal(stamp) || check.LastCheckTs.CheckValid() != nil {
			t.Fatal("timestamp instant or nanosecond precision changed")
		}
		if check.ConnectTimeout.AsDuration() != 1500*time.Millisecond || check.CheckTimeout.AsDuration() != 2500*time.Millisecond || check.DelayLoop.AsDuration() != 3*time.Second || check.RetryDelay.AsDuration() != 250*time.Millisecond {
			t.Fatal("duration units or subsecond precision changed")
		}
	}
}
