package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx         context.Context
	syslogSvc   *SyslogService
	stats       SystemStats
	statsMutex  sync.RWMutex
	startTime   time.Time
}

type SystemStats struct {
	TotalLogs      int64   `json:"totalLogs"`
	DeviceCount    int     `json:"deviceCount"`
	ServiceRunning bool    `json:"serviceRunning"`
	ListenPort     int     `json:"listenPort"`
	StartTime      string  `json:"startTime"`
	MemoryUsage    uint64  `json:"memoryUsage"`
	CPUUsage       float64 `json:"cpuUsage"`
	Connections    int     `json:"connections"`
	ReceiveRate    float64 `json:"receiveRate"`
	Protocol       string  `json:"protocol"`
	DatabaseSize   int64   `json:"databaseSize"`
}

func NewApp() *App {
	return &App{
		stats: SystemStats{
			ListenPort: 5140,
		},
		startTime: time.Now(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	GetDB()
	a.syslogSvc = NewSyslogService(a)
	a.stats.StartTime = time.Now().Format("2006-01-02 15:04:05")
	go a.startLogCleanupTask()
}

func (a *App) startLogCleanupTask() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		a.cleanupLogsIfNeeded()
	}
}

func (a *App) cleanupLogsIfNeeded() {
	db := GetDB()
	var config SystemConfig
	if err := db.First(&config).Error; err != nil {
		return
	}

	var logCount int64
	db.Model(&SyslogLog{}).Count(&logCount)

	retentionDays := config.LogRetention
	if retentionDays <= 0 {
		retentionDays = 7
	}

	if logCount > 10000 {
		cutoff := time.Now().AddDate(0, 0, -retentionDays)
		result := db.Where("received_at < ?", cutoff).Delete(&SyslogLog{})
		if result.RowsAffected > 0 {
			log.Printf("[CLEANUP] Deleted %d old logs (older than %d days)\n", result.RowsAffected, retentionDays)
			db.Exec("PRAGMA wal_checkpoint(PASSIVE)")
			db.Exec("VACUUM")
		}
	}

	var alertCount int64
	db.Model(&AlertRecord{}).Count(&alertCount)

	if alertCount > 10000 {
		cutoff := time.Now().AddDate(0, 0, -7)
		result := db.Where("created_at < ?", cutoff).Delete(&AlertRecord{})
		if result.RowsAffected > 0 {
			log.Printf("[CLEANUP] Deleted %d old alert records\n", result.RowsAffected)
		}
	}

	if logCount > 50000 {
		db.Exec("PRAGMA wal_checkpoint(PASSIVE)")
	}
}

func (a *App) GetSystemStats() SystemStats {
	a.statsMutex.RLock()
	defer a.statsMutex.RUnlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	a.stats.MemoryUsage = m.Alloc / 1024 / 1024

	dbPath := getDatabasePath()
	if info, err := os.Stat(dbPath); err == nil {
		a.stats.DatabaseSize = info.Size()
	}

	return a.stats
}

func getDatabasePath() string {
	dataDir := getDataDir()
	return filepath.Join(dataDir, "syslog.db")
}

func (a *App) UpdateStats(logs int64, devices int, running bool) {
	a.statsMutex.Lock()
	defer a.statsMutex.Unlock()
	a.stats.TotalLogs = logs
	a.stats.DeviceCount = devices
	a.stats.ServiceRunning = running
}

func (a *App) StartSyslogService(port int, protocol string) error {
	if a.syslogSvc == nil {
		a.syslogSvc = NewSyslogService(a)
	}
	a.stats.ListenPort = port
	return a.syslogSvc.Start(port, protocol)
}

func (a *App) StopSyslogService() error {
	if a.syslogSvc != nil {
		return a.syslogSvc.Stop()
	}
	return nil
}

func (a *App) GetLocalIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	
	var physicalIP string
	var otherIP string
	
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		
		name := strings.ToLower(iface.Name)
		isVirtual := strings.Contains(name, "vnic") || 
			strings.Contains(name, "vmnet") || 
			strings.Contains(name, "bridge") ||
			strings.Contains(name, "veth") ||
			strings.Contains(name, "docker") ||
			strings.Contains(name, "vEthernet") ||
			strings.Contains(name, "parallels") ||
			strings.HasPrefix(name, "utun") ||
			strings.HasPrefix(name, "awdl") ||
			strings.HasPrefix(name, "llw") ||
			strings.HasPrefix(name, "anpi") ||
			strings.HasPrefix(name, "vnic") ||
			strings.HasPrefix(name, "vmnet")
		
		isPhysical := strings.HasPrefix(name, "en") || 
			strings.HasPrefix(name, "eth") || 
			strings.HasPrefix(name, "wlan") ||
			strings.HasPrefix(name, "wi-fi")
		
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ip := ipnet.IP.String()
					if isPhysical && !isVirtual {
						if physicalIP == "" {
							physicalIP = ip
						}
					} else if !isVirtual && otherIP == "" {
						otherIP = ip
					}
				}
			}
		}
	}
	
	if physicalIP != "" {
		return physicalIP
	}
	if otherIP != "" {
		return otherIP
	}
	
	return "127.0.0.1"
}

func (a *App) FormatSyslogMessage(msg string) map[string]string {
	result := make(map[string]string)

	parts := strings.SplitN(msg, " ", 6)
	if len(parts) >= 5 {
		result["priority"] = parts[0]
		result["timestamp"] = parts[1]
		result["hostname"] = parts[2]
		result["app"] = parts[3]
		result["pid"] = parts[4]
		if len(parts) > 5 {
			result["message"] = parts[5]
		}
	}
	result["raw"] = msg
	return result
}

func (a *App) TestDingTalkWebhook(webhookURL, secret string) (string, error) {
	return SendDingTalkTestMessage(webhookURL, secret)
}

func (a *App) TestFeishuWebhook(webhookURL, secret string) (string, error) {
	return SendFeishuTestMessage(webhookURL, secret)
}

func (a *App) TestWeworkWebhook(webhookURL, key string) (string, error) {
	return SendWeworkTestMessage(webhookURL, key)
}

func (a *App) TestEmail(host string, port int, username, password, from, to string) (string, error) {
	return SendEmailTestMessage(host, port, username, password, from, to)
}

func (a *App) TestSyslogForward(host string, port int, protocol string, format string) (string, error) {
	err := TestSyslogForward(host, port, protocol, format)
	if err != nil {
		return "", err
	}
	return "测试消息发送成功！", nil
}

func (a *App) GetAppVersion() string {
	return "1.3.3"
}

func (a *App) GetPlatformInfo() string {
	return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
}

func (a *App) WindowMinimise() {
	wailsRuntime.WindowMinimise(a.ctx)
}

func (a *App) WindowMaximise() {
	wailsRuntime.WindowMaximise(a.ctx)
}

func (a *App) WindowClose() {
	wailsRuntime.Quit(a.ctx)
}
