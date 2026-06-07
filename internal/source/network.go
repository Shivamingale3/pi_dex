package source

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"syscall"
	"time"
	"unsafe"

	"github.com/Shivamingale3/pi_dex/internal/core"
)

type NetworkSource struct {
	bus *core.EventBus
}

func NewNetworkSource(bus *core.EventBus) *NetworkSource {
	return &NetworkSource{bus: bus}
}

func (s *NetworkSource) Run(ctx context.Context) error {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, syscall.NETLINK_ROUTE)
	if err != nil {
		log.Printf("network: socket: %v", err)
		return err
	}
	defer syscall.Close(fd)

	addr := &syscall.SockaddrNetlink{
		Family: syscall.AF_NETLINK,
		Groups: 1, // RTMGRP_LINK
	}
	if err := syscall.Bind(fd, addr); err != nil {
		log.Printf("network: bind: %v", err)
		return err
	}

	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		n, _, err := syscall.Recvfrom(fd, buf, 0)
		if err != nil {
			log.Printf("network: recv: %v", err)
			time.Sleep(time.Second)
			continue
		}

		msgs, err := syscall.ParseNetlinkMessage(buf[:n])
		if err != nil {
			continue
		}

		for _, msg := range msgs {
			event := s.parseLink(msg)
			if event != nil {
				s.bus.Publish(*event)
			}
		}
	}
}

type ifinfomsg struct {
	Family uint8
	_      uint8
	Type   uint16
	Index  int32
	Flags  uint32
	Change uint32
}

func (s *NetworkSource) parseLink(msg syscall.NetlinkMessage) *core.Event {
	if msg.Header.Type != syscall.RTM_NEWLINK && msg.Header.Type != syscall.RTM_DELLINK {
		return nil
	}

	if len(msg.Data) < int(unsafe.Sizeof(ifinfomsg{})) {
		return nil
	}

	ifim := (*ifinfomsg)(unsafe.Pointer(&msg.Data[0]))
	ifindex := ifim.Index
	attrs := parseRtAttrs(msg.Data[unsafe.Sizeof(ifinfomsg{}):])
	name, _ := attrs["IFLA_IFNAME"].(string)
	if name == "" {
		name = fmt.Sprintf("if%d", ifindex)
	}

	ts := time.Now()

	if msg.Header.Type == syscall.RTM_DELLINK {
		return &core.Event{
			Source:    core.SourceNetwork,
			EventType: core.EventInterfaceDown,
			Severity:  core.SeverityWarn,
			Title:     "Interface Removed",
			Message:   name + " removed",
			Timestamp: ts,
		}
	}

	operstate, _ := attrs["IFLA_OPERSTATE"].(string)
	switch operstate {
	case "UP":
		return &core.Event{
			Source:    core.SourceNetwork,
			EventType: core.EventInterfaceUp,
			Severity:  core.SeverityWarn,
			Title:     "Interface Up",
			Message:   name + " is UP",
			Timestamp: ts,
		}
	case "DOWN":
		return &core.Event{
			Source:    core.SourceNetwork,
			EventType: core.EventInterfaceDown,
			Severity:  core.SeverityWarn,
			Title:     "Interface Down",
			Message:   name + " is DOWN",
			Timestamp: ts,
		}
	}

	return nil
}

func parseRtAttrs(data []byte) map[string]any {
	attrs := make(map[string]any)
	for i := 0; i+4 <= len(data); {
		length := int(binary.LittleEndian.Uint16(data[i : i+2]))
		rtype := int(binary.LittleEndian.Uint16(data[i+2 : i+4]))
		if length < 4 {
			break
		}
		if i+length > len(data) {
			break
		}
		payload := data[i+4 : i+length]
		// Pad to 4 bytes
		_ = payload
		switch rtype {
		case 3: // IFLA_IFNAME
			attrs["IFLA_IFNAME"] = cString(payload)
		case 16: // IFLA_OPERSTATE
			if len(payload) > 0 {
				attrs["IFLA_OPERSTATE"] = cString(payload[:1])
			}
		}
		i += ralign(length)
	}
	return attrs
}

func ralign(n int) int {
	return (n + 3) & ^3
}

func cString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
