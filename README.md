# go-sliding-window

An implementation of the sliding window protocol (SWP) in Go,
for reliable and ordered delivery of a sequence of messages.

Variations on this algorithm are used in TCP/IP for reliability,
ordering, and flow-control.

Reference: pp118-120, Computer Networks: A Systems Approach
  by Peterson and Davie, Morgan Kaufmann Publishers, 1996.

Per Peterson and Davie, the SWP has three aims:

a. SWP reliably delivers messages across an unreliable link. SWP accomplishes this by acknowledging messages, and automatically resending messages that do not get acknowledged within a timeout.

b. SWP preserves the order in which messages are transmitted and received, by attaching sequence numbers and holding off on delivery until ordered delivery is obtained.

c. SWP can provide flow control. Overly fast senders can be throttled by slower receivers.

