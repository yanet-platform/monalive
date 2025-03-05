# Monalive

Monalive is a health checker that monitors the availability of real servers and
dynamically manages service announcements based on their status. It is designed
to integrate with the YANET load balancer, using its protobuf-based binary
protocol to communicate with the control plane.

For service announcements, Monalive works with a modified version of BIRD. The
patched repository is available here: <https://github.com/yanet-platform/bird>.

To accurately replicate the packet path from the load balancer to the host,
Monalive uses a tunneling mechanism.

## Features

- Health checking for real servers
- Service announcement control based on server availability
- Integration with YANET load balancer based on Protocol Buffers communication
  with the control plane
- Tunneling mechanism for end-to-end packet path replication

Supported health check types:

- TCP connection
- HTTP/HTTPS get request
- gRPC (with SSL enabled)

## Installation

Monalive requires **Go 1.22** or **later**.

To build the application, clone the repository and run:

```sh
make setup
```

This command will:

- Install Protocol Buffers and gRPC dependencies
- Download third-party packages
- Generate code for those packages
- Build the application for Linux on the amd64 architecture

## Configuration

Monalive's configuration is divided into two parts:

1. Application configuration – defines global parameters such as logging, gRPC
   and HTTP endpoints, update intervals, and other settings.
2. Services configuration – specifies virtual and real servers to be monitored,
   using a Keepalived-like syntax.

### Application Configuration

This file defines various parameters such as logging policies, gRPC and HTTP
endpoints, announcer and balancer update intervals, and more. See
[monalive-example.yaml](etc/monalive/monalive-example.yaml) for details.

### Services Configuration

Monalive uses a Keepalived-like syntax to configure virtual and real servers.
You can find an example of the configuration file in
[services-example.conf](etc/monalive/services-example.conf).

#### Virtual Server

The following parameters are supported for virtual servers and have the same
types and meanings as in Keepalived:

- `ip` – Virtual server's IP address.
- `port` – Port on which the virtual server operates.
- `protocol` – Protocol used (e.g., TCP, UDP).
- `lvs_sched` – LVS scheduling algorithm.
- `lvs_method` – LVS forwarding method.
- `quorum` – Required quorum value to maintain server availability.
- `hysteresis` – Hysteresis value to prevent frequent state changes.
- `virtualhost` – Virtual hostname associated with the server.
- `ops` – Enables one-packet-scheduler (OPS) for UDP load balancing.
- `delay_loop` – Interval between health checks.
- `retry`, `nb_get_retry` – Number of health check retry attempts.
- `delay_before_retry` – Delay between retry attempts.
- `real_server` – List of real servers backing the virtual server.

Additionally, Monalive introduces the following parameters:

- `announce_group` (string) – Specifies the prefix group to which the virtual
server IP address belongs.
- `version` (string, optional) – Tracks the configuration version.

#### Real Server

Similar to the Virtual Server, most parameters match those in Keepalived:

- `ip` – Real server's IP address.
- `port` – Port on which the real server operates.
- `weight` – Weight assigned to the real server for load balancing.
- `inhibit_on_failure` – If enabled, a failed health check sets the server's
weight to zero instead of removing it.
- `virtualhost` – Optional virtual hostname for the real server.
- `lvs_method` – Forwarding method used for health checks (e.g., TUN, GRE).
- `delay_loop` – Interval between health checks.
- `retry`, `nb_get_retry` – Number of health check retry attempts.
- `delay_before_retry` – Delay between retry attempts.

#### Check

Monalive supports multiple health check parameters to determine the availability
of real servers.

- `path` – URL path used for the health check. [HTTP, HTTPS]
- `status_code` – Expected HTTP status code for a successful check. [HTTP,
  HTTPS]
- `digest` – Digest for response validation. [HTTP, HTTPS]
- `virtualhost` – Optional field specifying the virtual host for HTTP/HTTPS
  checks. Also used as the gRPC service name in gRPC checks. [HTTP, HTTPS, gRPC]
- `connect_ip` – IP address used to connect to the service. [HTTP, HTTPS, gRPC,
  TCP]
- `connect_port` – Port used to connect to the service. [HTTP, HTTPS, gRPC, TCP]
- `bindto` – Local IP address for outgoing connections. [HTTP, HTTPS, gRPC, TCP]
- `connect_timeout` – Timeout for establishing a connection (in seconds). [HTTP,
  HTTPS, gRPC, TCP]
- `check_timeout` – Total timeout for the health check, including response wait
  time (in seconds). [HTTP, HTTPS, gRPC]
- `delay_loop` – Interval between health checks. [HTTP, HTTPS, gRPC, TCP]
- `retry`, `nb_get_retry` – Number of health check retry attempts. [HTTP, HTTPS,
  gRPC, TCP]
- `delay_before_retry` – Delay between retry attempts. [HTTP, HTTPS, gRPC, TCP]
- `dynamic_weight_enable` – Enables dynamic weight adjustment based on check
  results. [HTTP, HTTPS, gRPC]
- `dynamic_weight_in_header` – Determines if dynamic weighting is based on HTTP
  headers or body. [HTTP, HTTPS, gRPC]
- `dynamic_weight_coefficient` – Coefficient (percentage) for calculating weight
  adjustments. [HTTP, HTTPS, gRPC]

## Host Configuration

Monalive, for tunneling health checks, marks packets with `fwmark = 0xFFFFFFFF`.
These packets must then be routed to `NFQUEUE` so that the `checktun` daemon,
which is part of Monalive, can read and process them. To ensure proper routing
of the packet back to userspace for processing, the following host
configurations are necessary:

For IPv4 (`iptables`):

```sh
iptables -t mangle -A OUTPUT -m mark --mark 0xFFFFFFFF -j NFQUEUE --queue-num 1
```

For IPv6 (`ip6tables`):

```sh
ip6tables -t mangle -A OUTPUT -m mark --mark 0xFFFFFFFF -j NFQUEUE --queue-num 1
```

These commands ensure that any packet with the specified mark (`0xFFFFFFFF`)
will be redirected to queue number `1` for userspace processing via `NFQUEUE`.

## License

[Apache License, Version 2.0](LICENSE)
