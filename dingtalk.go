package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	stdlog "log"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type DingTalkMessage struct {
	MsgType  string            `json:"msgtype"`
	Markdown *DingTalkMarkdown `json:"markdown,omitempty"`
	Text     *DingTalkText     `json:"text,omitempty"`
}

type DingTalkMarkdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type DingTalkText struct {
	Content string `json:"content"`
}

type DingTalkResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func SendDingTalkMessage(webhookURL, secret, content string) error {
	if secret != "" {
		timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
		sign := generateSign(timestamp, secret)
		webhookURL = fmt.Sprintf("%s&timestamp=%s&sign=%s", webhookURL, timestamp, url.QueryEscape(sign))
	}

	message := DingTalkMessage{
		MsgType: "markdown",
		Markdown: &DingTalkMarkdown{
			Title: "Syslog告警",
			Text:  content,
		},
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	var result DingTalkResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	if result.ErrCode != 0 {
		return fmt.Errorf("dingtalk api error: %s", result.ErrMsg)
	}

	return nil
}

func SendDingTalkTestMessage(webhookURL, secret string) (string, error) {
	if secret != "" {
		timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
		sign := generateSign(timestamp, secret)
		webhookURL = fmt.Sprintf("%s&timestamp=%s&sign=%s", webhookURL, timestamp, url.QueryEscape(sign))
	}

	message := DingTalkMessage{
		MsgType: "text",
		Text: &DingTalkText{
			Content: "【测试消息】Syslog告警系统连接测试成功！\n\n发送时间: " + time.Now().Format("2006-01-02 15:04:05"),
		},
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return "", fmt.Errorf("failed to marshal message: %v", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	var result DingTalkResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("dingtalk api error: %s", result.ErrMsg)
	}

	return "测试消息发送成功！", nil
}

func generateSign(timestamp, secret string) string {
	stringToSign := timestamp + "\n" + secret

	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))

	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// ========== 企业微信推送 ==========

func SendWeworkMessage(webhookURL, key, content string) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL is empty")
	}

	if key != "" {
		if strings.Contains(webhookURL, "?") {
			webhookURL = fmt.Sprintf("%s&key=%s", webhookURL, key)
		} else {
			webhookURL = fmt.Sprintf("%s?key=%s", webhookURL, key)
		}
	}

	message := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": content,
		},
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	if errcode, ok := result["errcode"]; ok && errcode.(float64) != 0 {
		errmsg, _ := result["errmsg"].(string)
		return fmt.Errorf("wework api error: %s", errmsg)
	}

	return nil
}

func SendWeworkTestMessage(webhookURL, key string) (string, error) {
	if webhookURL == "" {
		return "", fmt.Errorf("webhook URL is empty")
	}

	if key != "" {
		if strings.Contains(webhookURL, "?") {
			webhookURL = fmt.Sprintf("%s&key=%s", webhookURL, key)
		} else {
			webhookURL = fmt.Sprintf("%s?key=%s", webhookURL, key)
		}
	}

	message := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": "【测试消息】Syslog告警系统连接测试成功！\n\n发送时间: " + time.Now().Format("2006-01-02 15:04:05"),
		},
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return "", fmt.Errorf("failed to marshal message: %v", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if errcode, ok := result["errcode"]; ok && errcode.(float64) != 0 {
		errmsg, _ := result["errmsg"].(string)
		return "", fmt.Errorf("wework api error: %s", errmsg)
	}

	return "测试消息发送成功！", nil
}

// ========== 飞书推送 ==========

type FeishuMessage struct {
	MsgType string        `json:"msg_type"`
	Content FeishuContent `json:"content"`
}

type FeishuContent struct {
	Text string      `json:"text,omitempty"`
	Post *FeishuPost `json:"post,omitempty"`
}

type FeishuPost struct {
	ZhCN FeishuPostContent `json:"zh_cn"`
}

type FeishuPostContent struct {
	Title   string                `json:"title"`
	Content [][]FeishuPostElement `json:"content"`
}

type FeishuPostElement struct {
	Tag  string `json:"tag"`
	Text string `json:"text,omitempty"`
}

type FeishuResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func SendFeishuMessage(webhookURL, secret, content string) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL is empty")
	}

	log.Printf("[DEBUG] SendFeishuMessage - webhookURL: %s, secret: %t\n", webhookURL, secret)

	if secret != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		sign := generateFeishuSign(timestamp, secret)
		log.Printf("[DEBUG] After sign processing - webhookURL: %s\n", webhookURL)
		if strings.Contains(webhookURL, "?") {
			webhookURL = fmt.Sprintf("%s&timestamp=%s&sign=%s", webhookURL, timestamp, url.QueryEscape(sign))
		} else {
			webhookURL = fmt.Sprintf("%s?timestamp=%s&sign=%s", webhookURL, timestamp, url.QueryEscape(sign))
		}
	}

	lines := strings.Split(content, "\n")
	var title string
	var postContent [][]FeishuPostElement

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "### ") {
			title = strings.TrimPrefix(line, "### ")
			continue
		}

		var elements []FeishuPostElement

		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				fieldName := strings.TrimSpace(parts[0])
				fieldValue := strings.TrimSpace(parts[1])

				fieldName = strings.ReplaceAll(fieldName, "**", "")
				fieldValue = strings.ReplaceAll(fieldValue, "**", "")

				elements = append(elements, FeishuPostElement{Tag: "text", Text: fieldName + ": " + fieldValue + "\n"})
			} else {
				line = strings.ReplaceAll(line, "**", "")
				elements = append(elements, FeishuPostElement{Tag: "text", Text: line + "\n"})
			}
		} else {
			line = strings.ReplaceAll(line, "**", "")
			elements = append(elements, FeishuPostElement{Tag: "text", Text: line + "\n"})
		}

		postContent = append(postContent, elements)
	}

	if title == "" {
		title = "安全告警"
	}

	message := FeishuMessage{
		MsgType: "post",
		Content: FeishuContent{
			Post: &FeishuPost{
				ZhCN: FeishuPostContent{
					Title:   title,
					Content: postContent,
				},
			},
		},
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	stdlog.Printf("[DEBUG] Final webhookURL: %s\n", webhookURL)
	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %v", err)
	}

	var result FeishuResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	if result.Code != 0 {
		return fmt.Errorf("feishu api error: %s", result.Msg)
	}

	return nil
}

func SendFeishuTestMessage(webhookURL, secret string) (string, error) {
	if webhookURL == "" {
		return "", fmt.Errorf("webhook URL is empty")
	}

	if secret != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		sign := generateFeishuSign(timestamp, secret)
		webhookURL = fmt.Sprintf("%s&timestamp=%s&sign=%s", webhookURL, timestamp, url.QueryEscape(sign))
	}

	message := FeishuMessage{
		MsgType: "text",
		Content: FeishuContent{
			Text: "【测试消息】Syslog告警系统连接测试成功！\n\n发送时间: " + time.Now().Format("2006-01-02 15:04:05"),
		},
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return "", fmt.Errorf("failed to marshal message: %v", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	var result FeishuResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if result.Code != 0 {
		return "", fmt.Errorf("feishu api error: %s", result.Msg)
	}

	return "测试消息发送成功！", nil
}

func generateFeishuSign(timestamp, secret string) string {
	stringToSign := timestamp + "\n" + secret

	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))

	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// ========== 邮箱推送 ==========

func SendEmailMessage(host string, port int, username, password, from, to, subject, content string) error {
	if host == "" || from == "" || to == "" {
		return fmt.Errorf("email configuration is incomplete")
	}

	recipients := strings.Split(to, ",")
	for i, r := range recipients {
		recipients[i] = strings.TrimSpace(r)
	}

	fromAddr, err := mail.ParseAddress(from)
	if err != nil {
		fromAddr = &mail.Address{Address: from}
	}

	msg := fmt.Sprintf("From: %s\r\n", fromAddr.String())
	msg += fmt.Sprintf("To: %s\r\n", to)
	msg += fmt.Sprintf("Subject: %s\r\n", subject)
	msg += "MIME-version: 1.0;\r\nContent-Type: text/plain; charset=\"UTF-8\";\r\n\r\n"
	msg += strings.ReplaceAll(content, "**", "")

	addr := fmt.Sprintf("%s:%d", host, port)

	var client *smtp.Client

	if port == 465 {
		tlsConfig := &tls.Config{
			ServerName: host,
		}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server (SSL): %v", err)
		}
		client, err = smtp.NewClient(conn, host)
		if err != nil {
			conn.Close()
			return fmt.Errorf("failed to create SMTP client: %v", err)
		}
	} else {
		client, err = smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server: %v", err)
		}

		if port == 587 {
			if err := client.StartTLS(nil); err != nil {
				client.Close()
				return fmt.Errorf("failed to start TLS: %v", err)
			}
		}
	}
	defer client.Close()

	var auth smtp.Auth
	if username != "" && password != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("authentication failed: %v", err)
		}
	}

	if err := client.Mail(fromAddr.Address); err != nil {
		return fmt.Errorf("failed to set sender: %v", err)
	}

	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to add recipient %s: %v", recipient, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to send data: %v", err)
	}

	_, err = w.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("failed to write message: %v", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data writer: %v", err)
	}

	return nil
}

func SendEmailTestMessage(host string, port int, username, password, from, to string) (string, error) {
	if host == "" || from == "" || to == "" {
		return "", fmt.Errorf("email configuration is incomplete")
	}

	subject := "【测试消息】Syslog告警系统连接测试"
	content := fmt.Sprintf("Syslog告警系统连接测试成功！\n\n发送时间: %s", time.Now().Format("2006-01-02 15:04:05"))

	err := SendEmailMessage(host, port, username, password, from, to, subject, content)
	if err != nil {
		return "", err
	}

	return "测试邮件发送成功！", nil
}
