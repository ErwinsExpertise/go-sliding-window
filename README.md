# go-sliding-window

[Docs: https://godoc.org/github.com/glycerine/go-sliding-window](https://godoc.org/github.com/glycerine/go-sliding-window)

## description

Package swp implements the same Sliding Window Protocol that
TCP uses for flow-control and reliable, ordered delivery.

The Nats event bus (https://nats.io/) is a
software model of a hardware multicast
switch. Nats provides multicast, but no guarantees of delivery
and no flow-control. This works fine as long as your
downstream read/subscribe capacity is larger than your
publishing rate.

If your nats publisher evers produces
faster than your subscriber can keep up, you may overrun
your buffers and drop messages. If your sender is local
and replaying a disk file of traffic over nats, you are
guanateed to exhaust even the largest of the internal
nats client buffers. In addition you may wish guaranteed
order of delivery (even with dropped messages), which
swp provides.

Hence swp was built to provide flow-control and reliable, ordered
delivery on top of the nats event bus. It reproduces the
TCP sliding window and flow-control mechanism in a
Session between two nats clients. It provides flow
control between exactly two nats endpoints; in many
cases this is sufficient to allow all subscribers to
keep up. If you have a wide variation in consumer
performance, establish the rate-controlling
`swp` Session between your producer and your slowest consumer.

There is also a Session.RegisterAsap() API that can be
used to obtain the same possibly-out-of-order but as-soon-as-possible
delivery that nats give you natively, while retaining the
flow control required to produce a lossless and ordered
delivery stream at the same time. This can be used in
tandem with the main always-ordered API if so desired.


## notes

An implementation of the sliding window protocol (SWP) in Go.

This algorithm is the same one that TCP uses for reliability,
ordering, and flow-control.

Reference: pp118-120, Computer Networks: A Systems Approach
  by Peterson and Davie, Morgan Kaufmann Publishers, 1996.
  For flow control implementation details see section 6.2.4
  "Sliding Window Revisted", pp296-299. For discussion
  of RTT estimation via Karn/Partridge or Jacobson/Karels
  see pp302-304.

Per Peterson and Davie, the SWP has three benefits:

 * SWP reliably delivers messages across an unreliable link. SWP accomplishes this by acknowledging messages, and automatically resending messages that do not get acknowledged within a timeout.

 * SWP preserves the order in which messages are transmitted and received, by attaching sequence numbers and holding off on delivery until ordered delivery is obtained.

 * SWP can provide flow control. Overly fast senders can be throttled by slower receivers. We implement this here; it was the main motivation for `swp` development.

### status

The library was test-driven and features a network simulator that simulates packet reordering, duplication, and loss. We pass tests with 20% packet loss easily. The library is usable now, but see the todo below. Also the network simulator could be improved by adding a chaos monkey mode that is even more aggressive about re-ordering and duplicating packets.

### todo

Optimal bandwidth allocation (when to timeout and retry) requires good estimates of the actual round-trip time between endpoints A and B in a Session, which may not be known in advance and may evolve over time. Currently this is just a user supplied parameter when creating a Session, and no exponential back-off is used. To make a session more convenient to configure however, we do plan to implement the [Jacobson/Karels algorithm for RTT estimation](https://en.wikipedia.org/wiki/TCP_congestion-avoidance_algorithm) in the near future (p304 of Peterson and Davie).

### credits

Author: Jason E. Aten, Ph.D.

License: MIT
