package keepalived

import (
	"bufio"
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Config struct {
	Services []*Service `keepalive:"virtual_server"`
}

type Service struct {
	VIP                    netip.Addr     `keepalive_pos:"0"`
	VPort                  *uint16        `keepalive_pos:"1"`
	Protocol               string         `keepalive:"protocol"`
	Scheduler              string         `keepalive:"lvs_sched"`
	ForwardingMethod       string         `keepalive:"lvs_method"`
	QuorumUp               string         `keepalive:"quorum_up"`
	QuorumDown             string         `keepalive:"quorum_down"`
	Quorum                 int            `keepalive:"quorum" default:"1"`
	Hysteresis             int            `keepalive:"hysteresis"`
	VirtualHost            string         `keepalive:"virtualhost"`
	CheckScheduler         CheckScheduler `keepalive_nested:"scheduler"`
	FwMark                 int            `keepalive:"fwmark"`
	OnePacketScheduler     bool           `keepalive:"ops"`
	IPv4OuterSourceNetwork string         `keepalive:"ipv4_outer_source_network"`
	IPv6OuterSourceNetwork string         `keepalive:"ipv6_outer_source_network"`
	Reals                  []*Real        `keepalive:"real_server"`
}

type Real struct {
	IP               netip.Addr     `keepalive_pos:"0"`
	Port             *uint16        `keepalive_pos:"1"`
	Weight           int            `keepalive:"weight" default:"1"`
	InhibitOnFailure bool           `keepalive:"inhibit_on_failure"`
	CheckScheduler   CheckScheduler `keepalive_nested:"scheduler"`
	HTTPCheckers     []*HTTPChecker `keepalive:"HTTP_GET"`
}

type CheckScheduler struct {
	DelayLoop  *int `keepalive:"delay_loop"`
	Retries    *int `keepalive:"retry,nb_get_retry"`
	RetryDelay *int `keepalive:"delay_before_retry"`
}

type CheckNet struct {
	ConnectIP      netip.Addr `keepalive:"connect_ip"`
	ConnectPort    uint16     `keepalive:"connect_port"`
	BindIP         netip.Addr `keepalive:"bindto"`
	ConnectTimeout float64    `keepalive:"connect_timeout"`
	CheckTimeout   float64    `keepalive:"check_timeout"`
	FwMark         int        `keepalive:"fwmark"`
}

type URL struct {
	Path       string `keepalive:"path"`
	StatusCode int    `keepalive:"status_code"`
	Digest     string `keepalive:"digest"`
}

type WeightControl struct {
	DynamicWeight       bool `keepalive:"dynamic_weight_enable"`
	DynamicWeightHeader bool `keepalive:"dynamic_weight_in_header"`
	DynamicWeightCoeff  int  `keepalive:"dynamic_weight_coefficient"`
}

type HTTPChecker struct {
	Scheduler     CheckScheduler `keepalive_nested:"scheduler"`
	Net           CheckNet       `keepalive_nested:"net"`
	URL           URL            `keepalive:"url"`
	WeightControl WeightControl  `keepalive_nested:"weight_control"`
}

func TestConfig(t *testing.T) {
	text := `#virtual_server 2001:dead:beef::1 80
virtual_server 2001:dead:beef::1 80 {
        protocol TCP
          
        quorum_up   "/etc/keepalived/quorum.sh up   2001:dead:beef::1,b-100,1"
        quorum_down "/etc/keepalived/quorum.sh down 2001:dead:beef::1,b-100,1"
        quorum 1
        hysteresis 0
          
        alpha
        omega
        lvs_method TUN
        lvs_sched wrr
        
        delay_loop 10
        virtualhost fqdn.example.com
        
        ops
        
        real_server 2001:dead:beef::2 80 {
                # RS: 2001:dead:beef::2
                weight 4
                inhibit_on_failure

                 HTTP_GET {
                        url {
                                path /
                                status_code 200
 	                               
                        }
                        connect_ip 2001:dead:beef::1
                        connect_port 80
                        bindto 2001:dead:beef::10
                        connect_timeout 1
                        fwmark 1111
                        
                        nb_get_retry 1
                        
                        delay_before_retry 1
                
                }
        }
        real_server 2001:dead:beef::3 80 {
                # RS: 2001:dead:beef::3
                weight 10
                
                HTTP_GET {
                        url {
                                path /
                                status_code 200
                                
                        }
                        connect_ip 2001:dead:beef::1
                        connect_port 80
                        bindto 2001:dead:beef::10
                        connect_timeout 1.1
                        fwmark 2222
                        
                        nb_get_retry 1
                        
                        delay_before_retry 1
          
                }
        }
        
}`

	refCfg := []confItem{
		{
			File:   "test.cfg",
			Name:   "virtual_server",
			Values: []string{"2001:dead:beef::1", "80"},
			SubItems: []confItem{
				{
					File:     "test.cfg",
					Name:     "protocol",
					Values:   []string{"TCP"},
					SubItems: []confItem(nil),
				},
				{
					File:     "test.cfg",
					Name:     "quorum_up",
					Values:   []string{"/etc/keepalived/quorum.sh up   2001:dead:beef::1,b-100,1"},
					SubItems: []confItem(nil),
				},
				{
					File:     "test.cfg",
					Name:     "quorum_down",
					Values:   []string{"/etc/keepalived/quorum.sh down 2001:dead:beef::1,b-100,1"},
					SubItems: []confItem(nil),
				},
				{
					File:     "test.cfg",
					Name:     "quorum",
					Values:   []string{"1"},
					SubItems: []confItem(nil),
				},
				{
					File:     "test.cfg",
					Name:     "hysteresis",
					Values:   []string{"0"},
					SubItems: []confItem(nil),
				},
				{
					File:     "test.cfg",
					Name:     "alpha",
					Values:   []string(nil),
					SubItems: []confItem(nil),
				},
				{
					File:     "test.cfg",
					Name:     "omega",
					Values:   []string(nil),
					SubItems: []confItem(nil),
				},
				{
					File:     "test.cfg",
					Name:     "lvs_method",
					Values:   []string{"TUN"},
					SubItems: []confItem(nil),
				},
				{
					File:     "test.cfg",
					Name:     "lvs_sched",
					Values:   []string{"wrr"},
					SubItems: []confItem(nil),
				},
				{
					File:     "test.cfg",
					Name:     "delay_loop",
					Values:   []string{"10"},
					SubItems: []confItem(nil),
				},
				{
					File:     "test.cfg",
					Name:     "virtualhost",
					Values:   []string{"fqdn.example.com"},
					SubItems: []confItem(nil),
				},
				{
					File:     "test.cfg",
					Name:     "ops",
					Values:   []string(nil),
					SubItems: []confItem(nil),
				},
				{
					File:   "test.cfg",
					Name:   "real_server",
					Values: []string{"2001:dead:beef::2", "80"},
					SubItems: []confItem{
						{
							File:     "test.cfg",
							Name:     "weight",
							Values:   []string{"4"},
							SubItems: []confItem(nil),
						},
						{
							File:     "test.cfg",
							Name:     "inhibit_on_failure",
							Values:   []string(nil),
							SubItems: []confItem(nil),
						},
						{
							File:   "test.cfg",
							Name:   "HTTP_GET",
							Values: []string(nil),
							SubItems: []confItem{
								{
									File:   "test.cfg",
									Name:   "url",
									Values: []string(nil),
									SubItems: []confItem{
										{
											File:     "test.cfg",
											Name:     "path",
											Values:   []string{"/"},
											SubItems: []confItem(nil),
										},
										{
											File:     "test.cfg",
											Name:     "status_code",
											Values:   []string{"200"},
											SubItems: []confItem(nil),
										},
									},
								},
								{
									File:     "test.cfg",
									Name:     "connect_ip",
									Values:   []string{"2001:dead:beef::1"},
									SubItems: []confItem(nil),
								},
								{
									File:     "test.cfg",
									Name:     "connect_port",
									Values:   []string{"80"},
									SubItems: []confItem(nil),
								},
								{
									File:     "test.cfg",
									Name:     "bindto",
									Values:   []string{"2001:dead:beef::10"},
									SubItems: []confItem(nil),
								},
								{
									File:     "test.cfg",
									Name:     "connect_timeout",
									Values:   []string{"1"},
									SubItems: []confItem(nil),
								},
								{
									File:     "test.cfg",
									Name:     "fwmark",
									Values:   []string{"1111"},
									SubItems: []confItem(nil),
								},
								{
									File:     "test.cfg",
									Name:     "nb_get_retry",
									Values:   []string{"1"},
									SubItems: []confItem(nil),
								},
								{
									File:     "test.cfg",
									Name:     "delay_before_retry",
									Values:   []string{"1"},
									SubItems: []confItem(nil),
								},
							},
						},
					},
				},
				{
					File:   "test.cfg",
					Name:   "real_server",
					Values: []string{"2001:dead:beef::3", "80"},
					SubItems: []confItem{
						{
							File:     "test.cfg",
							Name:     "weight",
							Values:   []string{"10"},
							SubItems: []confItem(nil),
						},
						{
							File:   "test.cfg",
							Name:   "HTTP_GET",
							Values: []string(nil),
							SubItems: []confItem{
								{
									File:   "test.cfg",
									Name:   "url",
									Values: []string(nil),
									SubItems: []confItem{
										{
											File:     "test.cfg",
											Name:     "path",
											Values:   []string{"/"},
											SubItems: []confItem(nil),
										},
										{
											File:     "test.cfg",
											Name:     "status_code",
											Values:   []string{"200"},
											SubItems: []confItem(nil),
										},
									},
								},
								{
									File:     "test.cfg",
									Name:     "connect_ip",
									Values:   []string{"2001:dead:beef::1"},
									SubItems: []confItem(nil),
								},
								{
									File:     "test.cfg",
									Name:     "connect_port",
									Values:   []string{"80"},
									SubItems: []confItem(nil),
								},
								{
									File:     "test.cfg",
									Name:     "bindto",
									Values:   []string{"2001:dead:beef::10"},
									SubItems: []confItem(nil),
								},
								{
									File:     "test.cfg",
									Name:     "connect_timeout",
									Values:   []string{"1.1"},
									SubItems: []confItem(nil),
								},
								{
									File:     "test.cfg",
									Name:     "fwmark",
									Values:   []string{"2222"},
									SubItems: []confItem(nil),
								},
								{
									File:     "test.cfg",
									Name:     "nb_get_retry",
									Values:   []string{"1"},
									SubItems: []confItem(nil),
								},
								{
									File:     "test.cfg",
									Name:     "delay_before_retry",
									Values:   []string{"1"},
									SubItems: []confItem(nil),
								},
							},
						},
					},
				},
			},
		},
	}
	unbalancedBraces := 0
	cfg, err := parseConfig(bufio.NewScanner(strings.NewReader(text)), "", "test.cfg", &unbalancedBraces)
	assert.NoError(t, err)
	assert.Equal(t, unbalancedBraces, 0)
	assert.Equal(t, refCfg, cfg)
}

func TestParseTokenHasUnexpectedChars(t *testing.T) {
	var cfgRoot confItem
	var err error

	text := `
	virtual_server 2001:dead:beef::1 80 {
			protocol TCP
>>>>>>>>>>>>>>>>>>mergeconflict_1
			
			quorum_up   "/etc/keepalived/quorum.sh up   2001:dead:beef::1,b-100,1"
			quorum_down "/etc/keepalived/quorum.sh down 2001:dead:beef::1,b-100,1"
			quorum 1
			hysteresis 0
			
			lvs_sched wrr
			delay_loop 10
}`
	unbalancedBraces := 0
	cfgRoot.SubItems, err = parseConfig(bufio.NewScanner(strings.NewReader(text)), "", "test.cfg", &unbalancedBraces)

	assert.ErrorContains(t, err, "unexpected character", "expected a parsing error: unexpected character in the token")

	text = `
			virtual_server 2001:dead:beef::1 80 {
					protocol TCP
#>>>>>>>>>>>>>>>>>>mergeconflict_1

					quorum_up   "/etc/keepalived/quorum.sh up   2001:dead:beef::1,b-100,1"
					quorum_down "/etc/keepalived/quorum.sh down 2001:dead:beef::1,b-100,1"
					quorum 1

			?!*$%unexpected_characters_2
					hysteresis 0

					lvs_sched wrr
					delay_loop 10
		}`

	cfgRoot.SubItems, err = parseConfig(bufio.NewScanner(strings.NewReader(text)), "", "test.cfg", &unbalancedBraces)

	assert.ErrorContains(t, err, "unexpected character", "expected a parsing error: unexpected character in the token")

	text = `
	virtual_server 2001:dead:beef::1 80 {
			protocol TCP
#>>>>>>>>>>>>>>>>>>mergeconflict_1

			quorum_up   "/etc/keepalived/quorum.sh up   2001:dead:beef::1,b-100,1"
			quorum_down "/etc/keepalived/quorum.sh down 2001:dead:beef::1,b-100,1"
			quorum 1

#?!*$%unexpected_characters_2
			hysteresis 0

			lvs_sched wrr
			delay_loop 10
			n0-digits-a110wed value_will_not_be_read_anyway
}`

	cfgRoot.SubItems, err = parseConfig(bufio.NewScanner(strings.NewReader(text)), "", "test.cfg", &unbalancedBraces)

	assert.NoError(t, err)

}

func TestParseUnmatchedQuotes(t *testing.T) {
	text := `
	virtual_server 2001:dead:beef::1 80 {
			protocol TCP

			quorum_up   "/etc/keepalived/quorum.sh up   2001:dead:beef::1,b-100,1"
			quorum_down "/etc/keepalived/quorum.sh down 2001:dead:beef::1,b-100,1"
			random_stuff "here quotation marks match" "and here they don't
			quorum 1
			hysteresis 0
			{

				lvs_sched wrr
				delay_loop 10
}`

	unbalancedBraces := 0
	_, err := parseConfig(bufio.NewScanner(strings.NewReader(text)), "", "test.cfg", &unbalancedBraces)

	assert.ErrorContains(t, err, "unmatched quotation mark", "expected a parsing error: unmatched quotation mark")

}

func TestParseUnbalancedBraces(t *testing.T) {
	text := `
	virtual_server 2001:dead:beef::1 80 {
			protocol TCP

			quorum_up   "/etc/keepalived/quorum.sh up   2001:dead:beef::1,b-100,1"
			quorum_down "/etc/keepalived/quorum.sh down 2001:dead:beef::1,b-100,1"
			quorum 1
			hysteresis 0
			some_complex_parameter {

				lvs_sched wrr
				delay_loop 10
}`

	unbalancedBraces := 0
	_, err := parseConfig(bufio.NewScanner(strings.NewReader(text)), "", "test.cfg", &unbalancedBraces)

	assert.NoError(t, err)
	fmt.Printf("Unbalanced braces %d\n", unbalancedBraces)
	assert.NotEqual(t, unbalancedBraces, 0)

	text = `
		virtual_server 2001:dead:beef::1 80 {
			protocol TCP

			quorum_up   "/etc/keepalived/quorum.sh up   2001:dead:beef::1,b-100,1"
			quorum_down "/etc/keepalived/quorum.sh down 2001:dead:beef::1,b-100,1"
			quorum 1
			hysteresis 0
			some_complex_parameter {

				lvs_sched wrr
				delay_loop 10
			}
		}
		some_other_parameter some_other_value
	}`

	unbalancedBraces = 0
	_, err = parseConfig(bufio.NewScanner(strings.NewReader(text)), "", "test.cfg", &unbalancedBraces)

	assert.NoError(t, err)
	fmt.Printf("Unbalanced braces %d\n", unbalancedBraces)
	assert.NotEqual(t, unbalancedBraces, 0)
}

func TestParseOpeningBraceInItemName(t *testing.T) {
	text := `
	virtual_server 2001:dead:beef::1 80 {
			protocol TCP

			quorum_up   "/etc/keepalived/quorum.sh up   2001:dead:beef::1,b-100,1"
			quorum_down "/etc/keepalived/quorum.sh down 2001:dead:beef::1,b-100,1"
			quorum 1
			hysteresis 0
			{

				lvs_sched wrr
				delay_loop 10
			}
}`

	unbalancedBraces := 0
	_, err := parseConfig(bufio.NewScanner(strings.NewReader(text)), "", "test.cfg", &unbalancedBraces)
	assert.ErrorContains(t, err, "cannot be a complex object", "expected a parsing error: parameter name cannot be a complex object")
}

func TestDecodeConfigPosNotTagged(t *testing.T) {
	text := `
	virtual_server 2001:dead:beef::1 80 pos_2_some_unexpected_text
	`

	cfgRoot := confItem{
		Path: "",
		File: "test.cfg",
	}
	var err error

	refCfg := []confItem{
		{
			File:     "test.cfg",
			Name:     "virtual_server",
			Values:   []string{"2001:dead:beef::1", "80", "pos_2_some_unexpected_text"},
			SubItems: []confItem(nil),
		},
	}

	unbalancedBraces := 0
	cfgRoot.SubItems, err = parseConfig(bufio.NewScanner(strings.NewReader(text)), "", "test.cfg", &unbalancedBraces)
	assert.NoError(t, err)
	assert.Equal(t, refCfg, cfgRoot.SubItems)

	config := Config{
		Services: []*Service{},
	}

	err = configDecode(&config, &cfgRoot)
	assert.ErrorContains(t, err, "with position id", "expected a decoding error: searching position tag")
}

func TestDefaultValueTag(t *testing.T) {
	text := `
    virtual_server 2001:dead:beef::1 80 {
            
            real_server 2001:dead:beef::2 80 {
                    weight 4
            }
            real_server 2001:dead:beef::3 80 {
                    weight
            }
			real_server 2001:dead:beef::4 80 {
					# field weight is empty
			}            
    }`
	cfgRoot := confItem{
		Path: "",
		File: "test.cfg",
	}

	var err error
	unbalancedBraces := 0
	cfgRoot.SubItems, err = parseConfig(bufio.NewScanner(strings.NewReader(text)), "", "test.cfg", &unbalancedBraces)
	assert.NoError(t, err)

	config := Config{
		Services: []*Service{},
	}

	err = configDecode(&config, &cfgRoot)
	assert.NoError(t, err)

	assert.Equal(t, config.Services[0].Reals[0].Weight, 4)
	assert.Equal(t, config.Services[0].Reals[1].Weight, 1)
	assert.Equal(t, config.Services[0].Reals[2].Weight, 1)
}

func TestOuterSourceNetwork(t *testing.T) {
	text := `
    virtual_server 2001:dead:beef::1 80 {
            
			ipv4_outer_source_network 123.0.0.12/32
            ipv6_outer_source_network 2001:dead:beef::/64
    }`
	cfgRoot := confItem{
		Path: "",
		File: "test.cfg",
	}

	var err error
	unbalancedBraces := 0
	cfgRoot.SubItems, err = parseConfig(bufio.NewScanner(strings.NewReader(text)), "", "test.cfg", &unbalancedBraces)
	assert.NoError(t, err)

	config := Config{
		Services: []*Service{},
	}

	err = configDecode(&config, &cfgRoot)
	assert.NoError(t, err)

	assert.Equal(t, config.Services[0].IPv4OuterSourceNetwork, "123.0.0.12/32")
	assert.Equal(t, config.Services[0].IPv6OuterSourceNetwork, "2001:dead:beef::/64")
}

func TestRetries(t *testing.T) {
	type testCase struct {
		text   string
		result int
	}
	testCases := []testCase{
		{
			text: `
				virtual_server 2001:dead:beef::1 80 {
						retry 10
				}`,
			result: 10,
		},
		{
			text: `
				virtual_server 2001:dead:beef::1 80 {
						nb_get_retry 5
						retry 10
				}`,
			result: 10,
		},
		{
			text: `
				virtual_server 2001:dead:beef::1 80 {
						nb_get_retry 5
				}`,
			result: 5,
		},
		{
			text: `
				virtual_server 2001:dead:beef::1 80 {
					retry 10
					nb_get_retry 5
				}`,
			result: 5,
		},
	}

	for id, test := range testCases {
		cfgRoot := confItem{
			Path: "",
			File: "test.cfg",
		}
		var err error
		unbalancedBraces := 0
		cfgRoot.SubItems, err = parseConfig(bufio.NewScanner(strings.NewReader(test.text)), "", "test.cfg", &unbalancedBraces)
		assert.NoError(t, err)

		config := Config{
			Services: []*Service{},
		}

		err = configDecode(&config, &cfgRoot)
		assert.NoError(t, err)

		assert.Equal(t, test.result, *config.Services[0].CheckScheduler.Retries, fmt.Sprintf("test number: %d", id))
	}
}
