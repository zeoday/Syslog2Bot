//go:build web
// +build web

package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

//go:embed all:frontend/dist
var webAssets embed.FS

type WebApp struct {
	startTime time.Time
	syslogSvc *SyslogService
	app       *App
}

var webApp *WebApp

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		printWebUsage()
		return
	}

	port := 8080
	if len(os.Args) > 1 {
		if os.Args[1] == "-p" && len(os.Args) > 2 {
			fmt.Sscanf(os.Args[2], "%d", &port)
		} else if !strings.HasPrefix(os.Args[1], "-") {
			fmt.Sscanf(os.Args[1], "%d", &port)
		}
	}

	webApp = &WebApp{
		startTime: time.Now(),
		app:       NewApp(),
	}
	webApp.app.startup(context.Background())

	mux := http.NewServeMux()

	mux.HandleFunc("/", webApp.handleStaticFiles)
	mux.HandleFunc("/api/", webApp.handleAPI)

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	ip := getLocalIP()
	url := fmt.Sprintf("http://%s:%d", ip, port)
	printBanner(url)

	go openBrowser(url)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()

	select {
	case <-quit:
		fmt.Println("\n正在停止服务...")
		webApp.app.StopSyslogService()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		server.Shutdown(ctx)
		cancel()
		fmt.Println("服务已停止")
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			log.Printf("服务器错误: %v\n", err)
		}
	}
}

func printWebUsage() {
	fmt.Println("Syslog2Bot Web Server v1.5.0")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  syslog2bot-web [port]           Start web server (default: 8080)")
	fmt.Println("  syslog2bot-web -p <port>        Start on specified port")
	fmt.Println("  syslog2bot-web --help           Show this help")
	fmt.Println("")
	fmt.Println("Access: http://localhost:<port>")
}

func printBanner(url string) {
	fmt.Printf("\n")
	fmt.Printf("╔════════════════════════════════════════════╗\n")
	fmt.Printf("║     Syslog2Bot Web Server v1.5.0           ║\n")
	fmt.Printf("║     By 迷人安全                            ║\n")
	fmt.Printf("╠════════════════════════════════════════════╣\n")
	fmt.Printf("║  URL: %s                            \n", url)
	fmt.Printf("║  Press Ctrl+C to stop                      ║\n")
	fmt.Printf("╚════════════════════════════════════════════╝\n\n")
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

func openBrowser(url string) {
	time.Sleep(500 * time.Millisecond)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	}
	if cmd != nil {
		cmd.Start()
	}
}

func (a *WebApp) handleStaticFiles(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}

	data, err := webAssets.ReadFile("frontend/dist" + path)
	if err != nil {
		data, _ = webAssets.ReadFile("frontend/dist/index.html")
	}

	contentType := "text/html"
	if strings.HasSuffix(path, ".js") {
		contentType = "application/javascript"
	} else if strings.HasSuffix(path, ".css") {
		contentType = "text/css"
	} else if strings.HasSuffix(path, ".png") {
		contentType = "image/png"
	} else if strings.HasSuffix(path, ".svg") {
		contentType = "image/svg+xml"
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(data)
}

func (a *WebApp) handleAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := strings.TrimPrefix(r.URL.Path, "/api/")

	switch {
	case path == "devices" && r.Method == "GET":
		json.NewEncoder(w).Encode(a.app.GetDevices())
	case strings.HasPrefix(path, "device/"):
		a.handleDevice(w, r, strings.TrimPrefix(path, "device/"))
	case path == "device-groups" && r.Method == "GET":
		json.NewEncoder(w).Encode(a.app.GetDeviceGroups())
	case strings.HasPrefix(path, "device-group/"):
		a.handleDeviceGroup(w, r, strings.TrimPrefix(path, "device-group/"))
	case path == "parse-templates" && r.Method == "GET":
		templates := a.app.GetParseTemplates()
		log.Printf("[API] GET /api/parse-templates, count: %d\n", len(templates))
		if len(templates) == 0 {
			log.Printf("[API] No parse templates found in database\n")
		}
		json.NewEncoder(w).Encode(templates)
	case strings.HasPrefix(path, "parse-template/"):
		a.handleParseTemplate(w, r, strings.TrimPrefix(path, "parse-template/"))
	case path == "filter-policies" && r.Method == "GET":
		policies := a.app.GetFilterPolicies()
		log.Printf("[API] GET /api/filter-policies, count: %d\n", len(policies))
		if len(policies) == 0 {
			log.Printf("[API] No filter policies found in database\n")
		}
		json.NewEncoder(w).Encode(policies)
	case strings.HasPrefix(path, "filter-policy/"):
		a.handleFilterPolicy(w, r, strings.TrimPrefix(path, "filter-policy/"))
	case path == "alert-policies" && r.Method == "GET":
		json.NewEncoder(w).Encode(a.app.GetAlertPolicies())
	case strings.HasPrefix(path, "alert-policy/"):
		a.handleAlertPolicy(w, r, strings.TrimPrefix(path, "alert-policy/"))
	case path == "robots" && r.Method == "GET":
		json.NewEncoder(w).Encode(a.app.GetRobots())
	case strings.HasPrefix(path, "robot/"):
		a.handleRobot(w, r, strings.TrimPrefix(path, "robot/"))
	case path == "test-robot" && r.Method == "POST":
		a.handleTestRobot(w, r)
	case strings.HasPrefix(path, "alert-rules/"):
		a.handleAlertRules(w, r, strings.TrimPrefix(path, "alert-rules/"))
	case strings.HasPrefix(path, "alert-rule/"):
		a.handleAlertRule(w, r, strings.TrimPrefix(path, "alert-rule/"))
	case path == "output-templates" && r.Method == "GET":
		json.NewEncoder(w).Encode(a.app.GetOutputTemplates())
	case strings.HasPrefix(path, "output-template/"):
		a.handleOutputTemplate(w, r, strings.TrimPrefix(path, "output-template/"))
	case path == "field-mapping-docs" && r.Method == "GET":
		json.NewEncoder(w).Encode(a.app.GetFieldMappingDocs())
	case strings.HasPrefix(path, "field-mapping-doc/"):
		a.handleFieldMappingDoc(w, r, strings.TrimPrefix(path, "field-mapping-doc/"))
	case path == "logs":
		a.handleLogs(w, r)
	case path == "logs/cleanup":
		a.handleLogsCleanup(w, r)
	case path == "service/status":
		a.handleServiceStatus(w, r)
	case path == "service/start":
		a.handleServiceStart(w, r)
	case path == "service/stop":
		a.handleServiceStop(w, r)
	case path == "config":
		a.handleConfig(w, r)
	case path == "stats":
		a.handleStats(w, r)
	case path == "field-stats" && r.Method == "POST":
		a.handleFieldStats(w, r)
	case strings.HasPrefix(path, "available-stats-fields/"):
		a.handleAvailableStatsFields(w, r, strings.TrimPrefix(path, "available-stats-fields/"))
	case path == "test-syslog":
		a.handleTestSyslog(w, r)
	case path == "test-syslog-forward":
		a.handleTestSyslogForward(w, r)
	case path == "test-parse":
		a.handleTestParse(w, r)
	case strings.HasPrefix(path, "log-trace/"):
		a.handleLogTrace(w, r, strings.TrimPrefix(path, "log-trace/"))
	case path == "local-ips":
		json.NewEncoder(w).Encode(a.app.GetLocalIPs())
	case path == "server-ip":
		json.NewEncoder(w).Encode(map[string]string{"ip": getLocalIP()})
	case path == "export/parse-templates":
		a.handleExportParseTemplates(w, r)
	case path == "export/filter-policies":
		a.handleExportFilterPolicies(w, r)
	case path == "import/parse-templates":
		a.handleImportParseTemplates(w, r)
	case path == "import/filter-policies":
		a.handleImportFilterPolicies(w, r)
	default:
		http.Error(w, `{"error": "not found"}`, 404)
	}
}

func (a *WebApp) handleDevice(w http.ResponseWriter, r *http.Request, idStr string) {
	switch r.Method {
	case "GET":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		device, err := a.app.GetDevice(uint(id))
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(device)
	case "POST":
		var device Device
		json.NewDecoder(r.Body).Decode(&device)
		log.Printf("[API] POST /api/device/0 - name: %s, ip: %s\n", device.Name, device.IPAddress)
		if err := a.app.AddDevice(device); err != nil {
			log.Printf("[API] AddDevice error: %v\n", err)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(device)
	case "PUT":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		var device Device
		json.NewDecoder(r.Body).Decode(&device)
		device.ID = uint(id)
		if err := a.app.UpdateDevice(device); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	case "DELETE":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		if err := a.app.DeleteDevice(uint(id)); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}
}

func (a *WebApp) handleDeviceGroup(w http.ResponseWriter, r *http.Request, idStr string) {
	switch r.Method {
	case "GET":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		group, err := a.app.GetDeviceGroup(uint(id))
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(group)
	case "POST":
		var group DeviceGroup
		json.NewDecoder(r.Body).Decode(&group)
		if err := a.app.AddDeviceGroup(group); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(group)
	case "PUT":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		var group DeviceGroup
		json.NewDecoder(r.Body).Decode(&group)
		group.ID = uint(id)
		if err := a.app.UpdateDeviceGroup(group); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	case "DELETE":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		if err := a.app.DeleteDeviceGroup(uint(id)); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}
}

func (a *WebApp) handleParseTemplate(w http.ResponseWriter, r *http.Request, idStr string) {
	switch r.Method {
	case "GET":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		template, err := a.app.GetParseTemplate(uint(id))
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(template)
	case "POST":
		var template ParseTemplate
		json.NewDecoder(r.Body).Decode(&template)
		if err := a.app.AddParseTemplate(template); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(template)
	case "PUT":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		var template ParseTemplate
		json.NewDecoder(r.Body).Decode(&template)
		template.ID = uint(id)
		if err := a.app.UpdateParseTemplate(template); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	case "DELETE":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		if err := a.app.DeleteParseTemplate(uint(id)); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}
}

func (a *WebApp) handleFilterPolicy(w http.ResponseWriter, r *http.Request, idStr string) {
	switch r.Method {
	case "GET":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		policy, err := a.app.GetFilterPolicy(uint(id))
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(policy)
	case "POST":
		var policy FilterPolicy
		json.NewDecoder(r.Body).Decode(&policy)
		if err := a.app.AddFilterPolicy(policy); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(policy)
	case "PUT":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		var policy FilterPolicy
		json.NewDecoder(r.Body).Decode(&policy)
		policy.ID = uint(id)
		if err := a.app.UpdateFilterPolicy(policy); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	case "DELETE":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		if err := a.app.DeleteFilterPolicy(uint(id)); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}
}

func (a *WebApp) handleAlertPolicy(w http.ResponseWriter, r *http.Request, idStr string) {
	switch r.Method {
	case "GET":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		policy, err := a.app.GetAlertPolicy(uint(id))
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(policy)
	case "POST":
		var policy AlertPolicy
		json.NewDecoder(r.Body).Decode(&policy)
		if err := a.app.AddAlertPolicy(policy); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(policy)
	case "PUT":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		var policy AlertPolicy
		json.NewDecoder(r.Body).Decode(&policy)
		policy.ID = uint(id)
		if err := a.app.UpdateAlertPolicy(policy); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	case "DELETE":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		if err := a.app.DeleteAlertPolicy(uint(id)); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}
}

func (a *WebApp) handleRobot(w http.ResponseWriter, r *http.Request, idStr string) {
	switch r.Method {
	case "GET":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		robot, err := a.app.GetRobot(uint(id))
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(robot)
	case "POST":
		var robot DingTalkRobot
		json.NewDecoder(r.Body).Decode(&robot)
		result, err := a.app.AddRobot(robot)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(result)
	case "PUT":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		var robot DingTalkRobot
		json.NewDecoder(r.Body).Decode(&robot)
		robot.ID = uint(id)
		if err := a.app.UpdateRobot(robot); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	case "DELETE":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		if err := a.app.DeleteRobot(uint(id)); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}
}

func (a *WebApp) handleTestRobot(w http.ResponseWriter, r *http.Request) {
	var robot DingTalkRobot
	json.NewDecoder(r.Body).Decode(&robot)

	var result string
	var err error

	switch robot.Platform {
	case "dingtalk":
		result, err = a.app.TestDingTalkWebhook(robot.WebhookURL, robot.Secret)
	case "feishu":
		result, err = a.app.TestFeishuWebhook(robot.FeishuWebhookURL, robot.FeishuSecret)
	case "wework":
		result, err = a.app.TestWeworkWebhook(robot.WeworkWebhookURL, robot.WeworkKey)
	case "email":
		result, err = a.app.TestEmail(robot.SMTPHost, robot.SMTPPort, robot.SMTPUsername, robot.SMTPPassword, robot.SMTPFrom, robot.SMTPTo)
	case "syslog":
		result, err = a.app.TestSyslogForward(robot.SyslogHost, robot.SyslogPort, robot.SyslogProtocol, robot.SyslogFormat)
	default:
		result, err = a.app.TestDingTalkWebhook(robot.WebhookURL, robot.Secret)
	}

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(result)
}

func (a *WebApp) handleOutputTemplate(w http.ResponseWriter, r *http.Request, idStr string) {
	switch r.Method {
	case "GET":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		template, err := a.app.GetOutputTemplate(uint(id))
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(template)
	case "POST":
		var template OutputTemplate
		json.NewDecoder(r.Body).Decode(&template)
		if err := a.app.AddOutputTemplate(template); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	case "PUT":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		var template OutputTemplate
		json.NewDecoder(r.Body).Decode(&template)
		template.ID = uint(id)
		if err := a.app.UpdateOutputTemplate(template); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	case "DELETE":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		if err := a.app.DeleteOutputTemplate(uint(id)); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}
}

func (a *WebApp) handleAlertRules(w http.ResponseWriter, r *http.Request, idStr string) {
	switch r.Method {
	case "GET":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		rules := a.app.GetAlertRules(uint(id))
		json.NewEncoder(w).Encode(rules)
	case "DELETE":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		if err := a.app.DeleteAlertRulesByRobotID(uint(id)); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}
}

func (a *WebApp) handleAlertRule(w http.ResponseWriter, r *http.Request, idStr string) {
	switch r.Method {
	case "GET":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		rule, err := a.app.GetAlertRule(uint(id))
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(rule)
	case "POST":
		var rule AlertRule
		json.NewDecoder(r.Body).Decode(&rule)
		if err := a.app.AddAlertRule(rule); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	case "PUT":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		var rule AlertRule
		json.NewDecoder(r.Body).Decode(&rule)
		rule.ID = uint(id)
		if err := a.app.UpdateAlertRule(rule); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	case "DELETE":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		if err := a.app.DeleteAlertRule(uint(id)); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}
}

func (a *WebApp) handleFieldMappingDoc(w http.ResponseWriter, r *http.Request, idStr string) {
	switch r.Method {
	case "GET":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		doc, err := a.app.GetFieldMappingDoc(uint(id))
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(doc)
	case "POST":
		var doc FieldMappingDoc
		json.NewDecoder(r.Body).Decode(&doc)
		if err := a.app.AddFieldMappingDoc(doc); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(doc)
	case "PUT":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		var doc FieldMappingDoc
		json.NewDecoder(r.Body).Decode(&doc)
		doc.ID = uint(id)
		if err := a.app.UpdateFieldMappingDoc(doc); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	case "DELETE":
		id, _ := strconv.ParseUint(idStr, 10, 32)
		if err := a.app.DeleteFieldMappingDoc(uint(id)); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}
}

func (a *WebApp) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
		if page == 0 {
			page = 1
		}
		if pageSize == 0 {
			pageSize = 50
		}

		log.Printf("[API] GET /api/logs - page=%d, pageSize=%d\n", page, pageSize)
		result := a.app.GetLogs(LogQueryParams{
			Page:     page,
			PageSize: pageSize,
		})
		log.Printf("[API] GetLogs result: total=%d, logs=%d\n", result.Total, len(result.Logs))
		json.NewEncoder(w).Encode(result)
	}
}

func (a *WebApp) handleLogsCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var params struct {
			Days int `json:"days"`
		}
		json.NewDecoder(r.Body).Decode(&params)
		err := a.app.CleanupLogs(params.Days)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}
}

func (a *WebApp) handleServiceStatus(w http.ResponseWriter, r *http.Request) {
	status := a.app.GetServiceStatus()
	log.Printf("[API] handleServiceStatus: serviceRunning=%v, listenPort=%v\n", status["serviceRunning"], status["listenPort"])
	json.NewEncoder(w).Encode(status)
}

func (a *WebApp) handleServiceStart(w http.ResponseWriter, r *http.Request) {
	log.Printf("[API] handleServiceStart called\n")
	var params struct {
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
	}
	json.NewDecoder(r.Body).Decode(&params)
	log.Printf("[API] Starting service on port %d with protocol %s\n", params.Port, params.Protocol)

	err := a.app.StartSyslogService(params.Port, params.Protocol)
	if err != nil {
		log.Printf("[API] StartSyslogService error: %v\n", err)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}
	log.Printf("[API] Service started successfully\n")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func (a *WebApp) handleServiceStop(w http.ResponseWriter, r *http.Request) {
	log.Printf("[API] handleServiceStop called\n")
	a.app.StopSyslogService()
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func (a *WebApp) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		config := a.app.GetConfig()
		json.NewEncoder(w).Encode(config)
	} else if r.Method == "PUT" {
		var config SystemConfig
		json.NewDecoder(r.Body).Decode(&config)
		a.app.SaveConfig(config)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}
}

func (a *WebApp) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := a.app.GetSystemStats()
	json.NewEncoder(w).Encode(stats)
}

func (a *WebApp) handleFieldStats(w http.ResponseWriter, r *http.Request) {
	var req FieldStatsRequest
	json.NewDecoder(r.Body).Decode(&req)
	result := a.app.GetFieldStats(req)
	json.NewEncoder(w).Encode(result)
}

func (a *WebApp) handleAvailableStatsFields(w http.ResponseWriter, r *http.Request, idStr string) {
	id, _ := strconv.ParseUint(idStr, 10, 32)
	fields := a.app.GetAvailableStatsFields(uint(id))
	json.NewEncoder(w).Encode(fields)
}

func (a *WebApp) handleTestSyslog(w http.ResponseWriter, r *http.Request) {
	var req TestSyslogRequest
	json.NewDecoder(r.Body).Decode(&req)
	result := a.app.SendTestSyslog(req)
	json.NewEncoder(w).Encode(result)
}

func (a *WebApp) handleTestSyslogForward(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	portStr := r.URL.Query().Get("port")
	protocol := r.URL.Query().Get("protocol")
	format := r.URL.Query().Get("format")

	port, _ := strconv.Atoi(portStr)

	result, err := a.app.TestSyslogForward(host, port, protocol, format)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(result)
}

func (a *WebApp) handleTestParse(w http.ResponseWriter, r *http.Request) {
	var req ParseTestRequest
	json.NewDecoder(r.Body).Decode(&req)
	result := a.app.TestParseTemplate(req)
	json.NewEncoder(w).Encode(result)
}

func (a *WebApp) handleLogTrace(w http.ResponseWriter, r *http.Request, idStr string) {
	id, _ := strconv.ParseUint(idStr, 10, 32)
	info := a.app.GetLogTraceInfo(uint(id))
	json.NewEncoder(w).Encode(info)
}

func (a *WebApp) handleExportParseTemplates(w http.ResponseWriter, r *http.Request) {
	var ids []uint
	json.NewDecoder(r.Body).Decode(&ids)
	result := a.app.ExportParseTemplates(ids)
	w.Write([]byte(result))
}

func (a *WebApp) handleExportFilterPolicies(w http.ResponseWriter, r *http.Request) {
	var ids []uint
	json.NewDecoder(r.Body).Decode(&ids)
	result := a.app.ExportFilterPolicies(ids)
	w.Write([]byte(result))
}

func (a *WebApp) handleImportParseTemplates(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	result := a.app.ImportParseTemplates(string(body))
	json.NewEncoder(w).Encode(result)
}

func (a *WebApp) handleImportFilterPolicies(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	result := a.app.ImportFilterPolicies(string(body))
	json.NewEncoder(w).Encode(result)
}
