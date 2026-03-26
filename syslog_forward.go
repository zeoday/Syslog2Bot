package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

type SyslogForwardMessage struct {
	Timestamp   string                 `json:"timestamp"`
	SourceIP    string                 `json:"sourceIp"`
	DeviceName  string                 `json:"deviceName"`
	Facility    int                    `json:"facility"`
	Severity    int                    `json:"severity"`
	Message     string                 `json:"message"`
	RawLog      string                 `json:"rawLog"`
	ParsedData  map[string]interface{} `json:"parsedData,omitempty"`
	Forwarded   bool                   `json:"forwarded"`
	ForwardedBy string                 `json:"forwardedBy,omitempty"`
}

func SendSyslogForward(host string, port int, protocol string, format string, message string, parsedData map[string]interface{}, log *SyslogLog) error {
	if host == "" || port == 0 {
		return fmt.Errorf("syslog host or port is empty")
	}

	var payload []byte
	var err error

	hostname, _ := os.Hostname()

	switch format {
	case "json":
		msg := SyslogForwardMessage{
			Timestamp:   time.Now().Format(time.RFC3339),
			SourceIP:    log.SourceIP,
			DeviceName:  log.DeviceName,
			Facility:    log.Facility,
			Severity:    log.Severity,
			Message:     message,
			RawLog:      log.RawMessage,
			ParsedData:  parsedData,
			Forwarded:   true,
			ForwardedBy: hostname,
		}
		payload, err = json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal json: %v", err)
		}
	case "rfc3164":
		ts := time.Now().Format("Jan 2 15:04:05")
		hostname := log.SourceIP
		if hostname == "" {
			hostname = "unknown"
		}
		payload = []byte(fmt.Sprintf("<134>%s %s syslog2bot: [FORWARDED] %s", ts, hostname, message))
	case "rfc5424":
		ts := time.Now().Format(time.RFC3339)
		hostname := log.SourceIP
		if hostname == "" {
			hostname = "unknown"
		}
		payload = []byte(fmt.Sprintf("<134>1 %s %s syslog2bot - - - [FORWARDED] %s", ts, hostname, message))
	default:
		msg := SyslogForwardMessage{
			Timestamp:   time.Now().Format(time.RFC3339),
			SourceIP:    log.SourceIP,
			DeviceName:  log.DeviceName,
			Facility:    log.Facility,
			Severity:    log.Severity,
			Message:     message,
			RawLog:      log.RawMessage,
			ParsedData:  parsedData,
			Forwarded:   true,
			ForwardedBy: hostname,
		}
		payload, err = json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal json: %v", err)
		}
	}

	address := fmt.Sprintf("%s:%d", host, port)

	protocol = strings.ToLower(protocol)
	if protocol == "" {
		protocol = "udp"
	}

	if protocol == "tcp" {
		conn, err := net.Dial("tcp", address)
		if err != nil {
			return fmt.Errorf("failed to connect to %s: %v", address, err)
		}
		defer conn.Close()
		_, err = conn.Write(payload)
		if err != nil {
			return fmt.Errorf("failed to send tcp message: %v", err)
		}
	} else {
		conn, err := net.Dial("udp", address)
		if err != nil {
			return fmt.Errorf("failed to connect to %s: %v", address, err)
		}
		defer conn.Close()
		_, err = conn.Write(payload)
		if err != nil {
			return fmt.Errorf("failed to send udp message: %v", err)
		}
	}

	return nil
}

func TestSyslogForward(host string, port int, protocol string, format string) error {
	if host == "" || port == 0 {
		return fmt.Errorf("syslog host or port is empty")
	}

	var payload []byte
	var err error

	switch format {
	case "rfc3164":
		ts := time.Now().Format("Jan 2 15:04:05")
		payload = []byte(fmt.Sprintf("<134>%s 127.0.0.1 syslog2bot: 【测试消息】Syslog2Bot连接测试成功！", ts))
	case "rfc5424":
		ts := time.Now().Format(time.RFC3339)
		payload = []byte(fmt.Sprintf("<134>1 %s 127.0.0.1 syslog2bot - - - 【测试消息】Syslog2Bot连接测试成功！", ts))
	default:
		testMessage := SyslogForwardMessage{
			Timestamp:  time.Now().Format(time.RFC3339),
			SourceIP:   "127.0.0.1",
			DeviceName: "syslog2bot",
			Facility:   1,
			Severity:   6,
			Message:    "【测试消息】Syslog2Bot连接测试成功！",
		}
		payload, err = json.Marshal(testMessage)
		if err != nil {
			return fmt.Errorf("failed to marshal json: %v", err)
		}
	}

	address := fmt.Sprintf("%s:%d", host, port)

	protocol = strings.ToLower(protocol)
	if protocol == "" {
		protocol = "udp"
	}

	if protocol == "tcp" {
		conn, err := net.Dial("tcp", address)
		if err != nil {
			return fmt.Errorf("failed to connect to %s: %v", address, err)
		}
		defer conn.Close()
		_, err = conn.Write(payload)
		if err != nil {
			return fmt.Errorf("failed to send tcp message: %v", err)
		}
	} else {
		conn, err := net.Dial("udp", address)
		if err != nil {
			return fmt.Errorf("failed to connect to %s: %v", address, err)
		}
		defer conn.Close()
		_, err = conn.Write(payload)
		if err != nil {
			return fmt.Errorf("failed to send udp message: %v", err)
		}
	}

	return nil
}
