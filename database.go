package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	db   *gorm.DB
	once sync.Once
)

func getDataDir() string {
	if envDataDir := os.Getenv("SYSLG_ALERT_DATA_DIR"); envDataDir != "" {
		return envDataDir
	}

	exePath, err := os.Executable()
	if err != nil {
		log.Printf("[DB] Failed to get executable path: %v, using './data'\n", err)
		return "./data"
	}
	exeDir := filepath.Dir(exePath)

	dataPath := filepath.Join(exeDir, "data")
	if _, err := os.Stat(dataPath); err == nil {
		return dataPath
	}

	if err := os.MkdirAll(dataPath, 0755); err != nil {
		log.Printf("[DB] Failed to create data directory: %v\n", err)
	} else {
		log.Printf("[DB] Created data directory: %s\n", dataPath)
	}
	return dataPath
}

func GetDataDir() string {
	return getDataDir()
}

func GetDB() *gorm.DB {
	once.Do(func() {
		var err error
		dataDir := getDataDir()
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			panic("Failed to create data directory: " + err.Error())
		}

		dbPath := filepath.Join(dataDir, "syslog.db")
		dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=10000&_sync=NORMAL&_cache_size=-64000", dbPath)
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			panic("Failed to connect database: " + err.Error())
		}

		sqlDB, err := db.DB()
		if err != nil {
			panic("Failed to get database connection: " + err.Error())
		}
		sqlDB.SetMaxOpenConns(5)
		sqlDB.SetMaxIdleConns(2)
		sqlDB.SetConnMaxLifetime(0)

		db.Exec("PRAGMA journal_mode = WAL")
		db.Exec("PRAGMA synchronous = NORMAL")
		db.Exec("PRAGMA cache_size = -64000")
		db.Exec("PRAGMA temp_store = MEMORY")
		db.Exec("PRAGMA mmap_size = 268435456")

		autoMigrate()
	})
	return db
}

func autoMigrate() {
	db.Exec("DROP INDEX IF EXISTS idx_field_mapping_docs_device_type")

	err := db.AutoMigrate(
		&DeviceGroup{},
		&Device{},
		&ParseTemplate{},
		&OutputTemplate{},
		&FilterPolicy{},
		&AlertPolicy{},
		&SyslogLog{},
		&Template{},
		&DingTalkRobot{},
		&AlertRule{},
		&AlertRecord{},
		&SystemConfig{},
		&FieldMappingDoc{},
	)
	if err != nil {
		panic("Failed to migrate database: " + err.Error())
	}

	ensureIndexes()
	initDefaultConfig()
	loadTemplatesFromDir()
	initDefaultFieldMappingDocs()
}

func loadTemplatesFromDir() {
	templatesDir := getTemplatesDir()
	log.Printf("[DB] Templates directory: %s\n", templatesDir)

	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		log.Printf("[DB] Templates directory not found: %s\n", templatesDir)
		return
	}

	parseTemplatesFile := filepath.Join(templatesDir, "parse_templates.json")
	log.Printf("[DB] Checking parse templates file: %s\n", parseTemplatesFile)
	if _, err := os.Stat(parseTemplatesFile); err == nil {
		log.Printf("[DB] Parse templates file exists, loading...\n")
		loadParseTemplatesFromFile(parseTemplatesFile)
	} else {
		log.Printf("[DB] Parse templates file not found or error: %v\n", err)
	}

	filterPoliciesFile := filepath.Join(templatesDir, "filter_policies.json")
	log.Printf("[DB] Checking filter policies file: %s\n", filterPoliciesFile)
	if _, err := os.Stat(filterPoliciesFile); err == nil {
		log.Printf("[DB] Filter policies file exists, loading...\n")
		loadFilterPoliciesFromFile(filterPoliciesFile)
	} else {
		log.Printf("[DB] Filter policies file not found or error: %v\n", err)
	}
}

func getTemplatesDir() string {
	if envDir := os.Getenv("SYSLG_ALERT_TEMPLATES_DIR"); envDir != "" {
		return envDir
	}

	exePath, err := os.Executable()
	if err != nil {
		log.Printf("[DB] Failed to get executable path: %v, using relative 'templates'\n", err)
		return "templates"
	}
	exeDir := filepath.Dir(exePath)
	log.Printf("[DB] Executable: %s\n", exePath)
	log.Printf("[DB] Exe directory: %s\n", exeDir)

	templatesPath := filepath.Join(exeDir, "templates")
	if _, err := os.Stat(templatesPath); err == nil {
		log.Printf("[DB] Found templates: %s\n", templatesPath)
		return templatesPath
	}

	log.Printf("[DB] Templates dir (default): %s\n", templatesPath)
	return templatesPath
}

func loadParseTemplatesFromFile(filePath string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("[DB] Failed to read parse templates file: %v\n", err)
		return
	}

	var data struct {
		Version   string `json:"version"`
		Templates []struct {
			Name           string `json:"name"`
			Description    string `json:"description"`
			ParseType      string `json:"parseType"`
			HeaderRegex    string `json:"headerRegex"`
			FieldMapping   string `json:"fieldMapping"`
			ValueTransform string `json:"valueTransform"`
			DeviceType     string `json:"deviceType"`
			IsActive       bool   `json:"isActive"`
		} `json:"templates"`
	}

	if err := json.Unmarshal(content, &data); err != nil {
		log.Printf("[DB] Failed to parse parse templates file: %v\n", err)
		return
	}

	log.Printf("[DB] Found %d parse templates in JSON file\n", len(data.Templates))

	for _, t := range data.Templates {
		var existing ParseTemplate
		result := db.Where("name = ?", t.Name).First(&existing)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				template := ParseTemplate{
					Name:           t.Name,
					Description:    t.Description,
					ParseType:      t.ParseType,
					HeaderRegex:    t.HeaderRegex,
					FieldMapping:   t.FieldMapping,
					ValueTransform: t.ValueTransform,
					DeviceType:     t.DeviceType,
					IsActive:       t.IsActive,
				}
				if err := db.Create(&template).Error; err != nil {
					log.Printf("[DB] Failed to create template %s: %v\n", t.Name, err)
				} else {
					log.Printf("[DB] Loaded parse template: %s\n", t.Name)
				}
			} else {
				log.Printf("[DB] Error checking template %s: %v\n", t.Name, result.Error)
			}
		} else {
			log.Printf("[DB] Template %s already exists, skipping\n", t.Name)
		}
	}
}

func loadFilterPoliciesFromFile(filePath string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("[DB] Failed to read filter policies file: %v\n", err)
		return
	}

	var data struct {
		Version  string `json:"version"`
		Policies []struct {
			Name              string `json:"name"`
			Description       string `json:"description"`
			ParseTemplateName string `json:"parseTemplateName"`
			Conditions        string `json:"conditions"`
			ConditionLogic    string `json:"conditionLogic"`
			Action            string `json:"action"`
			Priority          int    `json:"priority"`
			IsActive          bool   `json:"isActive"`
			DedupEnabled      bool   `json:"dedupEnabled"`
			DedupWindow       int    `json:"dedupWindow"`
			DropUnmatched     bool   `json:"dropUnmatched"`
		} `json:"policies"`
	}

	if err := json.Unmarshal(content, &data); err != nil {
		log.Printf("[DB] Failed to parse filter policies file: %v\n", err)
		return
	}

	for _, p := range data.Policies {
		var existing FilterPolicy
		result := db.Where("name = ?", p.Name).First(&existing)
		if result.Error == gorm.ErrRecordNotFound {
			var parseTemplateID uint
			var pt ParseTemplate
			if db.Where("name = ?", p.ParseTemplateName).First(&pt).Error == nil {
				parseTemplateID = pt.ID
			}

			policy := FilterPolicy{
				Name:            p.Name,
				Description:     p.Description,
				ParseTemplateID: parseTemplateID,
				Conditions:      p.Conditions,
				ConditionLogic:  p.ConditionLogic,
				Action:          p.Action,
				Priority:        p.Priority,
				IsActive:        p.IsActive,
				DedupEnabled:    p.DedupEnabled,
				DedupWindow:     p.DedupWindow,
				DropUnmatched:   p.DropUnmatched,
			}
			if err := db.Create(&policy).Error; err != nil {
				log.Printf("[DB] Failed to create policy %s: %v\n", p.Name, err)
			} else {
				log.Printf("[DB] Loaded filter policy: %s\n", p.Name)
			}
		}
	}
}

func ensureIndexes() {
	db.Exec("CREATE INDEX IF NOT EXISTS idx_syslog_logs_received_at ON syslog_logs(received_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_syslog_logs_device_id ON syslog_logs(device_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_syslog_logs_filter_status ON syslog_logs(filter_status)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_syslog_logs_device_filter ON syslog_logs(device_id, filter_status)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_alert_records_created_at ON alert_records(created_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_devices_enabled ON devices(enabled)")
	log.Printf("[DB] Indexes ensured\n")
}

func initDefaultConfig() {
	var config SystemConfig
	result := db.First(&config)
	if result.Error == gorm.ErrRecordNotFound {
		db.Create(&SystemConfig{
			ListenPort:            5140,
			LogRetention:          3,
			MaxLogSize:            524288000,
			AutoStart:             false,
			MinimizeToTray:        true,
			AlertEnabled:          true,
			AlertInterval:         60,
			UnmatchedLogRetention: 3,
			UnmatchedLogAlert:     true,
			DefaultFilterAction:   "keep",
			Theme:                 "dark",
			Language:              "zh-CN",
		})
	}
}

func initDefaultTemplates() {
	// 云锁解析模板 - 使用macApp中的配置
	var yunsuoCount int64
	db.Model(&ParseTemplate{}).Where("name = ?", "云锁-告警模板").Count(&yunsuoCount)
	if yunsuoCount == 0 {
		db.Create(&ParseTemplate{
			Name:           "云锁-告警模板",
			Description:    "解析云锁安全设备的syslog告警日志",
			ParseType:      "syslog_json",
			HeaderRegex:    `<(?P<priority>\d+)>(?P<timestamp>\w+ \d+ [\d:]+) (?P<hostname>\S+)[^{]*`,
			FieldMapping:   `{"alertTime":"告警时间","description":"事件描述","levelDesc":"威胁等级","threatType":"威胁类型","attackIp":"攻击IP","innerIp":"受害IP","machine.nickname":"系统名称","threatSource":"威胁来源","groupName":"分组名称","action.text":"告警详情","result":"防护状态","dealStatus":"处理状态"}`,
			ValueTransform: `{"result":{"0":"拦截","1":"未拦截"},"dealStatus":{"0":"未处理","1":"已处理（自动）","2":"已处理（手动）","3":"误报","4":"不关注","5":"处置失败","6":"处置中"}}`,
			DeviceType:     "云锁",
			IsActive:       true,
		})
	}

	// 天眼解析模板 - 使用macApp中的配置
	var tianyanCount int64
	db.Model(&ParseTemplate{}).Where("name = ?", "天眼-组合解析").Count(&tianyanCount)
	if tianyanCount == 0 {
		db.Create(&ParseTemplate{
			Name:           "天眼-组合解析",
			Description:    "解析天眼安全设备的告警日志，支持webids_alert、ips_alert和ioc_alert",
			ParseType:      "smart_delimiter",
			HeaderRegex:    "",
			FieldMapping:   `{"delimiter":"|!","typeField":0,"skipHeader":true,"headerRegex":"","subTemplates":{"webids_alert":{"alertNameField":3,"attackIPField":6,"victimIPField":8,"alertTimeField":4,"severityField":10,"attackResultField":26},"ips_alert":{"alertNameField":3,"attackIPField":6,"victimIPField":8,"alertTimeField":4,"severityField":10,"attackResultField":24},"ioc_alert":{"alertNameField":18,"attackIPField":6,"victimIPField":8,"alertTimeField":10,"severityField":12,"attackResultField":-1}}}`,
			ValueTransform: `{"severity":{"2":"低危","4":"中危","6":"高危","8":"危急"},"attackResult":{"0":"失败","1":"成功","2":"失陷","3":"失败"}}`,
			DeviceType:     "天眼",
			IsActive:       true,
		})
	}

	// 云锁输出模板 - 检查是否已存在
	var yunsuoOutputCount int64
	db.Model(&OutputTemplate{}).Where("device_type = ?", "云锁").Count(&yunsuoOutputCount)
	if yunsuoOutputCount == 0 {
		// 云锁输出模板
		db.Create(&OutputTemplate{
			Name:        "云锁-安全告警模板",
			Description: "云锁安全设备告警消息模板",
			Content: `### 🚨 云锁安全告警

**告警时间**: {{alertTime}}

**事件描述**: {{description}}

**威胁等级**: {{levelDesc}}

**威胁类型**: {{threatType}}

**攻击IP**: {{attackIp}}

**受害IP**: {{innerIp}}

**系统名称**: {{machine.nickname}}

**威胁来源**: {{threatSource}}

**分组名称**: {{groupName}}

**告警详情**: {{action.text}}

**防护状态**: {{result}}

**处理状态**: {{dealStatus}}`,
			Fields:     "alertTime,description,levelDesc,threatType,attackIp,innerIp,machine.nickname,threatSource,groupName,action.text,result,dealStatus",
			DeviceType: "云锁",
			IsActive:   true,
		})
	}

	// 天眼输出模板 - 检查是否已存在
	var tianyanOutputCount int64
	db.Model(&OutputTemplate{}).Where("device_type = ?", "天眼").Count(&tianyanOutputCount)
	if tianyanOutputCount == 0 {
		// 天眼输出模板
		db.Create(&OutputTemplate{
			Name:        "天眼-安全告警模板",
			Description: "天眼安全设备告警消息模板",
			Content: `### 🚨 天眼安全告警

**告警时间**: {{alertTime}}

**告警名称**: {{alertName}}

**攻击IP**: {{attackIP}}

**受害IP**: {{victimIP}}

**威胁等级**: {{severity}}

**攻击结果**: {{attackResult}}`,
			Fields:     "alertTime,alertName,attackIP,victimIP,severity,attackResult",
			DeviceType: "天眼",
			IsActive:   true,
		})
	}

	var count int64
	db.Model(&DeviceGroup{}).Count(&count)
	if count == 0 {
		db.Create(&DeviceGroup{
			Name:        "默认分组",
			Description: "默认设备分组",
			Color:       "#409eff",
			SortOrder:   0,
		})
	}
}

func initDefaultFieldMappingDocs() {
	// 云锁字段映射 - 检查是否已存在
	var yunsuoDocCount int64
	db.Model(&FieldMappingDoc{}).Where("device_type = ?", "云锁").Count(&yunsuoDocCount)
	if yunsuoDocCount == 0 {
		yunsuoMappings := `{
  "priority": "优先级",
  "timestamp": "时间戳",
  "hostname": "主机名",
  "accuracy": "告警精准度",
  "attackIp": "攻击者IP",
  "attackIpAddress": "攻击IP归属",
  "attackIpFlag": "攻击IP标记",
  "bannedStatus": "禁用状态",
  "categoryName": "分类名称",
  "categoryUuid": "分类UUID",
  "day": "日期",
  "dealStatus": "处理状态",
  "dealSuggestion": "处理建议",
  "dealTime": "处理时间",
  "description": "事件描述",
  "direction": "网络方向",
  "eventId": "事件ID",
  "eventUuid": "事件UUID",
  "groupName": "分组名称",
  "innerIp": "内网IP",
  "ip": "IP地址",
  "ipAddress": "IP归属",
  "levelDesc": "威胁等级",
  "localTimestamp": "本地时间",
  "loginUser": "登录用户",
  "logo": "产品标识",
  "machineUuid": "服务器UUID",
  "outerIp": "外网IP",
  "phase": "攻击阶段",
  "phaseDesc": "攻击阶段描述",
  "primarySource": "风险来源",
  "processingMethod": "处置类型",
  "result": "防护状态",
  "risk": "风险级别",
  "ruleDesc": "规则描述",
  "ruleId": "规则ID",
  "ruleName": "规则名称",
  "score": "风险评分",
  "serviceId": "服务ID",
  "source": "来源",
  "secondarySource": "二级来源",
  "sourceDesc": "来源描述",
  "standardTimestamp": "标准时间",
  "threatSource": "威胁来源",
  "threatType": "威胁类型",
  "typeName": "类型名称",
  "ucrc": "日志唯一值",
  "victimIpFlag": "受害IP标记",
  "action.text": "告警详情",
  "action.html": "告警HTML",
  "sourceIpAddress.city": "城市",
  "sourceIpAddress.country": "国家",
  "sourceIpAddress.ip": "来源IP",
  "sourceIpAddress.region": "省份",
  "sourceIpAddress.type": "网络类型",
  "machine.extranetIp": "外网IP",
  "machine.intranetIp": "内网IP",
  "machine.ipv4": "IPv4",
  "machine.ipv6": "IPv6",
  "machine.machineName": "服务器名称",
  "machine.onlineStatus": "在线状态",
  "machine.operatingSystem": "操作系统",
  "machine.osType": "系统类型",
  "machine.installTime": "安装时间",
  "machine.uuid": "服务器UUID",
  "subject.process": "进程名",
  "subject.user": "用户",
  "subject.pid": "进程ID",
  "subject.path": "进程路径",
  "subject.webPagePhysicalPath": "Web路径",
  "subject.procHash": "进程Hash",
  "object.ip": "目标IP",
  "object.port": "目标端口",
  "object.domain": "域名",
  "object.url": "URL",
  "object.path": "路径",
  "object.process": "进程",
  "object.cmdline": "命令行",
  "http.method": "请求方法",
  "http.url": "请求URL",
  "http.host": "目标主机",
  "http.userAgent": "UserAgent",
  "http.cookie": "Cookie",
  "http.referer": "Referer",
  "http.queryString": "查询参数"
}`
		db.Create(&FieldMappingDoc{
			Name:          "云锁字段映射",
			DeviceType:    "云锁",
			Description:   "云锁安全设备Syslog日志字段映射文档",
			FieldMappings: yunsuoMappings,
			IsActive:      true,
		})

		// 天眼字段映射
		tianyanMappings := `{
  "alertType": "告警类型",
  "alertName": "告警名称",
  "attackIP": "攻击IP",
  "victimIP": "受害IP",
  "alertTime": "告警时间",
  "severity": "威胁等级",
  "attackResult": "攻击结果",
  "srcIp": "源IP",
  "dstIp": "目标IP",
  "srcPort": "源端口",
  "dstPort": "目标端口",
  "protocol": "协议",
  "appProtocol": "应用协议",
  "url": "请求URL",
  "userAgent": "UserAgent",
  "method": "请求方法",
  "cookie": "Cookie",
  "referer": "Referer",
  "statusCode": "响应码",
  "responseSize": "响应大小",
  "requestSize": "请求大小",
  "country": "攻击IP归属国家",
  "city": "攻击IP归属城市",
  "isp": "攻击IP运营商",
  "latitude": "纬度",
  "longitude": "经度",
  "detail": "告警详情",
  "ruleName": "规则名称",
  "ruleId": "规则ID",
  "signatureId": "签名ID",
  "generatorId": "生成器ID",
  "classificationId": "分类ID",
  "priority": "优先级",
  "revision": "修订版本",
  "timestamp": "时间戳",
  "hostname": "设备主机名",
  "deviceIp": "设备IP",
  "deviceName": "设备名称",
  "sensorId": "传感器ID",
  "sensorName": "传感器名称"
}`
		db.Create(&FieldMappingDoc{
			Name:          "天眼字段映射",
			DeviceType:    "天眼",
			Description:   "天眼安全设备Syslog日志字段映射文档",
			FieldMappings: tianyanMappings,
			IsActive:      true,
		})
	}

	// 天眼字段映射 - 检查是否已存在
	var tianyanDocCount int64
	db.Model(&FieldMappingDoc{}).Where("device_type = ?", "天眼").Count(&tianyanDocCount)
	if tianyanDocCount == 0 {
		// 天眼字段映射
		tianyanMappings := `{
  "alertType": "告警类型",
  "alertName": "告警名称",
  "attackIP": "攻击IP",
  "victimIP": "受害IP",
  "alertTime": "告警时间",
  "severity": "威胁等级",
  "attackResult": "攻击结果",
  "srcIp": "源IP",
  "dstIp": "目标IP",
  "srcPort": "源端口",
  "dstPort": "目标端口",
  "protocol": "协议",
  "appProtocol": "应用协议",
  "url": "请求URL",
  "userAgent": "UserAgent",
  "method": "请求方法",
  "cookie": "Cookie",
  "referer": "Referer",
  "statusCode": "响应码",
  "responseSize": "响应大小",
  "requestSize": "请求大小",
  "country": "攻击IP归属国家",
  "city": "攻击IP归属城市",
  "isp": "攻击IP运营商",
  "latitude": "纬度",
  "longitude": "经度",
  "detail": "告警详情",
  "ruleName": "规则名称",
  "ruleId": "规则ID",
  "signatureId": "签名ID",
  "generatorId": "生成器ID",
  "classificationId": "分类ID",
  "priority": "优先级",
  "revision": "修订版本",
  "timestamp": "时间戳",
  "hostname": "设备主机名",
  "deviceIp": "设备IP",
  "deviceName": "设备名称",
  "sensorId": "传感器ID",
  "sensorName": "传感器名称"
}`
		db.Create(&FieldMappingDoc{
			Name:          "天眼字段映射",
			DeviceType:    "天眼",
			Description:   "天眼安全设备Syslog日志字段映射文档",
			FieldMappings: tianyanMappings,
			IsActive:      true,
		})
	}
}

func GetSystemConfig() SystemConfig {
	var config SystemConfig
	db.First(&config)
	config.DataDir = getDataDir()
	config.ConfigDir = getConfigDir()
	return config
}

func getConfigDir() string {
	if envConfigDir := os.Getenv("SYSLG_ALERT_CONFIG_DIR"); envConfigDir != "" {
		return envConfigDir
	}

	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		parentDir := filepath.Dir(exeDir)
		configPath := filepath.Join(parentDir, "templates")
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
		configPath2 := filepath.Join(exeDir, "templates")
		if _, err := os.Stat(configPath2); err == nil {
			return configPath2
		}
		return configPath
	}

	return "./templates"
}

func UpdateSystemConfig(config SystemConfig) error {
	return db.Save(&config).Error
}

func CreateDeviceGroup(group *DeviceGroup) error {
	return db.Create(group).Error
}

func GetDeviceGroups() []DeviceGroup {
	var groups []DeviceGroup
	db.Order("sort_order ASC").Find(&groups)
	return groups
}

func GetDeviceGroupByID(id uint) (*DeviceGroup, error) {
	var group DeviceGroup
	err := db.First(&group, id).Error
	return &group, err
}

func UpdateDeviceGroup(group *DeviceGroup) error {
	return db.Save(group).Error
}

func DeleteDeviceGroup(id uint) error {
	return db.Delete(&DeviceGroup{}, id).Error
}

func CreateDevice(device *Device) error {
	return db.Create(device).Error
}

func GetDevices() []Device {
	var devices []Device
	db.Find(&devices)
	return devices
}

func GetDeviceByID(id uint) (*Device, error) {
	var device Device
	err := db.First(&device, id).Error
	return &device, err
}

func GetDeviceByIP(ip string) (*Device, error) {
	var device Device
	err := db.Where("ip_address = ?", ip).First(&device).Error
	return &device, err
}

func UpdateDevice(device *Device) error {
	return db.Save(device).Error
}

func DeleteDevice(id uint) error {
	return db.Delete(&Device{}, id).Error
}

func CreateParseTemplate(template *ParseTemplate) error {
	return db.Create(template).Error
}

func GetParseTemplates() []ParseTemplate {
	var templates []ParseTemplate
	db.Find(&templates)
	return templates
}

func GetParseTemplateByID(id uint) (*ParseTemplate, error) {
	var template ParseTemplate
	err := db.First(&template, id).Error
	return &template, err
}

func UpdateParseTemplate(template *ParseTemplate) error {
	return db.Save(template).Error
}

func DeleteParseTemplate(id uint) error {
	return db.Delete(&ParseTemplate{}, id).Error
}

func CreateOutputTemplate(template *OutputTemplate) error {
	return db.Create(template).Error
}

func GetOutputTemplates() []OutputTemplate {
	var templates []OutputTemplate
	db.Find(&templates)
	return templates
}

func GetOutputTemplateByID(id uint) (*OutputTemplate, error) {
	var template OutputTemplate
	err := db.First(&template, id).Error
	return &template, err
}

func GetOutputTemplateByPlatform(platform string) (*OutputTemplate, error) {
	var template OutputTemplate
	err := db.Where("platform = ? AND is_active = ?", platform, true).First(&template).Error
	return &template, err
}

func UpdateOutputTemplate(template *OutputTemplate) error {
	return db.Save(template).Error
}

func DeleteOutputTemplate(id uint) error {
	return db.Delete(&OutputTemplate{}, id).Error
}

func CreateFilterPolicy(policy *FilterPolicy) error {
	return db.Create(policy).Error
}

func GetFilterPolicies() []FilterPolicy {
	var policies []FilterPolicy
	db.Order("priority DESC").Find(&policies)
	return policies
}

func GetFilterPoliciesByDeviceID(deviceID uint) []FilterPolicy {
	var policies []FilterPolicy
	db.Where("device_id = ? OR device_id = 0", deviceID).Order("priority DESC").Find(&policies)
	return policies
}

func GetFilterPoliciesByDeviceGroupID(groupID uint) []FilterPolicy {
	var policies []FilterPolicy
	db.Where("device_group_id = ? OR device_group_id = 0", groupID).Order("priority DESC").Find(&policies)
	return policies
}

func GetFilterPolicyByID(id uint) (*FilterPolicy, error) {
	var policy FilterPolicy
	err := db.First(&policy, id).Error
	return &policy, err
}

func UpdateFilterPolicy(policy *FilterPolicy) error {
	return db.Save(policy).Error
}

func DeleteFilterPolicy(id uint) error {
	return db.Delete(&FilterPolicy{}, id).Error
}

func CreateAlertPolicy(policy *AlertPolicy) error {
	return db.Create(policy).Error
}

func GetAlertPolicies() []AlertPolicy {
	var policies []AlertPolicy
	db.Find(&policies)
	return policies
}

func GetAlertPolicyByID(id uint) (*AlertPolicy, error) {
	var policy AlertPolicy
	err := db.First(&policy, id).Error
	return &policy, err
}

func UpdateAlertPolicy(policy *AlertPolicy) error {
	return db.Save(policy).Error
}

func DeleteAlertPolicy(id uint) error {
	return db.Delete(&AlertPolicy{}, id).Error
}

func CreateLog(log *SyslogLog) error {
	return db.Create(log).Error
}

func GetLogs(page, pageSize int, deviceID *int, startTime, endTime, keyword string) ([]SyslogLog, int64) {
	var logs []SyslogLog
	var total int64

	query := db.Model(&SyslogLog{})

	if deviceID != nil && *deviceID > 0 {
		query = query.Where("device_id = ?", *deviceID)
	}
	if startTime != "" {
		query = query.Where("received_at >= ?", startTime)
	}
	if endTime != "" {
		query = query.Where("received_at <= ?", endTime)
	}
	if keyword != "" {
		searchPattern := "%" + keyword + "%"
		query = query.Where("raw_message LIKE ? OR parsed_fields LIKE ?", searchPattern, searchPattern)
	}

	query.Count(&total)
	query.Order("received_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)

	return logs, total
}

func GetUnmatchedLogsCount() int64 {
	var count int64
	db.Model(&SyslogLog{}).Where("filter_status = ?", "unmatched").Count(&count)
	return count
}

func CreateTemplate(template *Template) error {
	return db.Create(template).Error
}

func GetTemplates() []Template {
	var templates []Template
	db.Find(&templates)
	return templates
}

func GetTemplateByID(id uint) (*Template, error) {
	var template Template
	err := db.First(&template, id).Error
	return &template, err
}

func UpdateTemplate(template *Template) error {
	return db.Save(template).Error
}

func DeleteTemplate(id uint) error {
	return db.Delete(&Template{}, id).Error
}

func CreateRobot(robot *DingTalkRobot) error {
	return db.Create(robot).Error
}

func GetRobots() []DingTalkRobot {
	var robots []DingTalkRobot
	db.Find(&robots)
	return robots
}

func GetRobotByID(id uint) (*DingTalkRobot, error) {
	var robot DingTalkRobot
	err := db.First(&robot, id).Error
	return &robot, err
}

func UpdateRobot(robot *DingTalkRobot) error {
	return db.Save(robot).Error
}

func DeleteRobot(id uint) error {
	return db.Delete(&DingTalkRobot{}, id).Error
}

func CreateAlertRecord(record *AlertRecord) error {
	return db.Create(record).Error
}

func GetAlertRecords(page, pageSize int) ([]AlertRecord, int64) {
	var records []AlertRecord
	var total int64

	db.Model(&AlertRecord{}).Count(&total)
	db.Order("sent_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records)

	return records, total
}

func GetLogCount() int64 {
	var count int64
	db.Model(&SyslogLog{}).Count(&count)
	return count
}

func GetDeviceCount() int64 {
	var count int64
	db.Model(&Device{}).Count(&count)
	return count
}

func GetMatchedLogCount() int64 {
	var count int64
	db.Model(&SyslogLog{}).Where("filter_status = ?", "matched").Count(&count)
	return count
}

func GetAlertCount() int64 {
	var count int64
	db.Model(&AlertRecord{}).Where("status = ?", "sent").Count(&count)
	return count
}

func CleanupOldLogs(days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)

	result := db.Where("received_at < ?", cutoff).Delete(&SyslogLog{})
	if result.Error != nil {
		return result.Error
	}

	log.Printf("[CLEANUP] Deleted %d old logs (cutoff: %s)\n", result.RowsAffected, cutoff.Format("2006-01-02 15:04:05"))

	if result.RowsAffected > 1000 {
		go func() {
			sqlDB, _ := db.DB()
			if sqlDB != nil {
				sqlDB.Exec("PRAGMA wal_checkpoint(FULL)")
				sqlDB.Exec("VACUUM")
				log.Printf("[CLEANUP] Database vacuum completed (deleted %d rows)\n", result.RowsAffected)
			}
		}()
	}
	return nil
}

func CleanupUnmatchedLogs(days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)
	result := db.Where("filter_status = ? AND received_at < ?", "unmatched", cutoff).Delete(&SyslogLog{})
	if result.Error != nil {
		return result.Error
	}

	log.Printf("[CLEANUP] Deleted %d unmatched logs (cutoff: %s)\n", result.RowsAffected, cutoff.Format("2006-01-02 15:04:05"))

	if result.RowsAffected > 1000 {
		go func() {
			sqlDB, _ := db.DB()
			if sqlDB != nil {
				sqlDB.Exec("PRAGMA wal_checkpoint(FULL)")
				sqlDB.Exec("VACUUM")
				log.Printf("[CLEANUP] Unmatched vacuum completed\n")
			}
		}()
	}
	return nil
}

func CleanupAllLogs() error {
	result := db.Exec("DELETE FROM syslog_logs")
	if result.Error != nil {
		return result.Error
	}
	log.Printf("[CLEANUP] Deleted all logs\n")
	db.Exec("PRAGMA wal_checkpoint(PASSIVE)")
	db.Exec("VACUUM")
	log.Printf("[CLEANUP] Database vacuum completed\n")
	return nil
}

func GetActiveAlertPolicies() []AlertPolicy {
	var policies []AlertPolicy
	db.Where("is_active = ?", true).Find(&policies)
	return policies
}

func GetAlertPoliciesByFilterPolicyID(filterPolicyID uint) []AlertPolicy {
	var policies []AlertPolicy
	db.Where("filter_policy_id = ? AND is_active = ?", filterPolicyID, true).Find(&policies)
	return policies
}

func GetRobotsByFilterPolicyID(filterPolicyID uint) []DingTalkRobot {
	var rules []AlertRule
	db.Where("filter_policy_id = ? AND is_active = ?", filterPolicyID, true).Find(&rules)

	var robotIDs []uint
	for _, rule := range rules {
		robotIDs = append(robotIDs, rule.RobotID)
	}

	if len(robotIDs) == 0 {
		return []DingTalkRobot{}
	}

	var robots []DingTalkRobot
	db.Where("id IN ? AND is_active = ?", robotIDs, true).Find(&robots)
	return robots
}

func UpdateLogFilterStatus(logID uint, status string, policyID uint) error {
	return db.Model(&SyslogLog{}).Where("id = ?", logID).Updates(map[string]interface{}{
		"filter_status":     status,
		"matched_policy_id": policyID,
	}).Error
}

func DeleteLog(logID uint) error {
	return db.Delete(&SyslogLog{}, logID).Error
}

func UpdateLogAlertStatus(logID uint, status string, policyID uint) error {
	return db.Model(&SyslogLog{}).Where("id = ?", logID).Updates(map[string]interface{}{
		"alert_status":    status,
		"alert_policy_id": policyID,
	}).Error
}

func GetAlertRulesByRobotID(robotID uint) []AlertRule {
	var rules []AlertRule
	db.Where("robot_id = ? AND is_active = ?", robotID, true).Order("created_at ASC").Find(&rules)
	return rules
}

func GetAlertRulesByFilterPolicyID(filterPolicyID uint) []AlertRule {
	var rules []AlertRule
	db.Where("filter_policy_id = ? AND is_active = ?", filterPolicyID, true).Find(&rules)
	return rules
}

func CreateAlertRule(rule *AlertRule) error {
	return db.Create(rule).Error
}

func UpdateAlertRule(rule *AlertRule) error {
	return db.Save(rule).Error
}

func DeleteAlertRule(id uint) error {
	return db.Delete(&AlertRule{}, id).Error
}

func DeleteAlertRulesByRobotID(robotID uint) error {
	return db.Where("robot_id = ?", robotID).Delete(&AlertRule{}).Error
}

func GetAlertRuleByID(id uint) (*AlertRule, error) {
	var rule AlertRule
	err := db.First(&rule, id).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func UpdateLogParsedFields(logID uint, parsedData, parsedFields string) error {
	return db.Model(&SyslogLog{}).Where("id = ?", logID).Updates(map[string]interface{}{
		"parsed_data":   parsedData,
		"parsed_fields": parsedFields,
	}).Error
}

func CreateFieldMappingDoc(doc *FieldMappingDoc) error {
	return db.Create(doc).Error
}

func GetFieldMappingDocs() []FieldMappingDoc {
	var docs []FieldMappingDoc
	db.Order("device_type ASC").Find(&docs)
	return docs
}

func GetFieldMappingDocByID(id uint) (*FieldMappingDoc, error) {
	var doc FieldMappingDoc
	err := db.First(&doc, id).Error
	return &doc, err
}

func GetFieldMappingDocByDeviceType(deviceType string) (*FieldMappingDoc, error) {
	var doc FieldMappingDoc
	err := db.Where("device_type = ?", deviceType).First(&doc).Error
	return &doc, err
}

func GetFieldMappingDocByName(name string) (*FieldMappingDoc, error) {
	var doc FieldMappingDoc
	err := db.Where("name = ?", name).First(&doc).Error
	return &doc, err
}

func UpdateFieldMappingDoc(doc *FieldMappingDoc) error {
	return db.Save(doc).Error
}

func DeleteFieldMappingDoc(id uint) error {
	return db.Delete(&FieldMappingDoc{}, id).Error
}

func GetFieldStats(req FieldStatsRequest) FieldStatsResult {
	result := FieldStatsResult{
		Field: req.Field,
	}

	query := db.Model(&SyslogLog{}).Session(&gorm.Session{})

	if req.FilterPolicyID > 0 {
		query = query.Where("matched_policy_id = ?", req.FilterPolicyID)
	}
	if req.StartTime != "" {
		query = query.Where("received_at >= ?", req.StartTime)
	}
	if req.EndTime != "" {
		query = query.Where("received_at <= ?", req.EndTime)
	}

	var totalLogs int64
	query.Count(&totalLogs)
	result.TotalLogs = totalLogs

	if req.TopN <= 0 {
		req.TopN = 10
	}

	type FieldCount struct {
		Value    string `json:"value"`
		Count    int64  `json:"count"`
		LastSeen string `json:"lastSeen"`
	}

	var fieldCounts []FieldCount

	fieldExpr := fmt.Sprintf("json_extract(parsed_data, '$.%s')", req.Field)

	baseQuery := db.Model(&SyslogLog{}).
		Where("parsed_data IS NOT NULL AND parsed_data != ''").
		Where(fieldExpr+" IS NOT NULL").
		Where(fieldExpr + " != ''")

	if req.FilterPolicyID > 0 {
		baseQuery = baseQuery.Where("matched_policy_id = ?", req.FilterPolicyID)
	}
	if req.StartTime != "" {
		baseQuery = baseQuery.Where("received_at >= ?", req.StartTime)
	}
	if req.EndTime != "" {
		baseQuery = baseQuery.Where("received_at <= ?", req.EndTime)
	}

	var fieldTotal int64
	baseQuery.Count(&fieldTotal)

	rows, err := baseQuery.
		Select(
			fieldExpr+" as value",
			"COUNT(*) as count",
			"MAX(received_at) as last_seen",
		).
		Group(fieldExpr).
		Order("count DESC").
		Limit(req.TopN).
		Rows()

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var fc FieldCount
			if err := rows.Scan(&fc.Value, &fc.Count, &fc.LastSeen); err == nil {
				fieldCounts = append(fieldCounts, fc)
			}
		}
	}

	result.Items = make([]StatsItem, 0, len(fieldCounts))

	var uniqueCount int64
	countQuery := db.Model(&SyslogLog{}).
		Where("parsed_data IS NOT NULL AND parsed_data != ''").
		Where(fieldExpr+" IS NOT NULL").
		Where(fieldExpr + " != ''")
	if req.FilterPolicyID > 0 {
		countQuery = countQuery.Where("matched_policy_id = ?", req.FilterPolicyID)
	}
	if req.StartTime != "" {
		countQuery = countQuery.Where("received_at >= ?", req.StartTime)
	}
	if req.EndTime != "" {
		countQuery = countQuery.Where("received_at <= ?", req.EndTime)
	}
	countQuery.Distinct(fieldExpr).Count(&uniqueCount)
	result.UniqueCount = uniqueCount

	for _, fc := range fieldCounts {
		percent := "0%"
		if fieldTotal > 0 {
			p := float64(fc.Count) / float64(fieldTotal) * 100
			percent = fmt.Sprintf("%.1f%%", p)
		}

		result.Items = append(result.Items, StatsItem{
			Value:    fc.Value,
			Location: "",
			Count:    fc.Count,
			Percent:  percent,
			LastSeen: fc.LastSeen,
		})
	}

	return result
}

func GetAvailableStatsFields(policyID uint) []StatsField {
	if policyID == 0 {
		return []StatsField{}
	}

	var policy FilterPolicy
	if err := db.First(&policy, policyID).Error; err != nil {
		return []StatsField{}
	}

	fieldMap := make(map[string]string)
	hasSubTemplates := false

	if policy.ParseTemplateID > 0 {
		var parseTemplate ParseTemplate
		if err := db.First(&parseTemplate, policy.ParseTemplateID).Error; err == nil {
			if parseTemplate.ParseType == "smart_delimiter" && parseTemplate.FieldMapping != "" {
				var fieldMappingData map[string]interface{}
				if err := json.Unmarshal([]byte(parseTemplate.FieldMapping), &fieldMappingData); err == nil {
					if subTemplatesRaw, ok := fieldMappingData["subTemplates"]; ok {
						hasSubTemplates = true
						if subTemplatesMap, ok := subTemplatesRaw.(map[string]interface{}); ok {
							fieldKeyMap := map[string]string{
								"alertNameField":    "alertName",
								"attackIPField":     "attackIP",
								"victimIPField":     "victimIP",
								"alertTimeField":    "alertTime",
								"severityField":     "severity",
								"attackResultField": "attackResult",
							}
							displayNameMap := map[string]string{
								"alertName":    "告警名称",
								"attackIP":     "攻击IP",
								"victimIP":     "受害IP",
								"alertTime":    "告警时间",
								"severity":     "威胁等级",
								"attackResult": "攻击结果",
							}
							for _, subRaw := range subTemplatesMap {
								if sub, ok := subRaw.(map[string]interface{}); ok {
									for fieldKey, fieldName := range fieldKeyMap {
										if _, exists := sub[fieldKey]; exists {
											fieldMap[fieldName] = displayNameMap[fieldName]
										}
									}
								}
							}
						}
					}
				}
			} else if parseTemplate.FieldMapping != "" {
				var simpleMapping map[string]string
				if err := json.Unmarshal([]byte(parseTemplate.FieldMapping), &simpleMapping); err == nil {
					for fieldName, displayName := range simpleMapping {
						if fieldName != "" {
							fieldMap[fieldName] = displayName
						}
					}
				} else {
					var complexMapping map[string]map[string]interface{}
					if err := json.Unmarshal([]byte(parseTemplate.FieldMapping), &complexMapping); err == nil {
						for targetField := range complexMapping {
							if targetField != "" {
								fieldMap[targetField] = targetField
							}
						}
					}
				}
			}

			if parseTemplate.SubTemplates != "" {
				var subTemplates []SubTemplateConfig
				if err := json.Unmarshal([]byte(parseTemplate.SubTemplates), &subTemplates); err == nil {
					hasSubTemplates = true
					displayNameMap := map[string]string{
						"alertName":    "告警名称",
						"attackIp":     "攻击IP",
						"victimIp":     "受害IP",
						"alertTime":    "告警时间",
						"severity":     "威胁等级",
						"attackResult": "攻击结果",
					}
					for _, st := range subTemplates {
						if st.AlertNameField > 0 {
							fieldMap["alertName"] = displayNameMap["alertName"]
						}
						if st.AttackIPField > 0 {
							fieldMap["attackIp"] = displayNameMap["attackIp"]
						}
						if st.VictimIPField > 0 {
							fieldMap["victimIp"] = displayNameMap["victimIp"]
						}
						if st.AlertTimeField > 0 {
							fieldMap["alertTime"] = displayNameMap["alertTime"]
						}
						if st.SeverityField > 0 {
							fieldMap["severity"] = displayNameMap["severity"]
						}
						if st.AttackResultField > 0 {
							fieldMap["attackResult"] = displayNameMap["attackResult"]
						}
						for _, cf := range st.CustomFields {
							if cf.Name != "" {
								fieldMap[cf.Name] = cf.Name
							}
						}
					}
				}
			}
		}
	}

	var fields []StatsField

	if hasSubTemplates {
		fixedFields := []string{"alertName", "attackIP", "victimIP", "alertTime", "severity", "attackResult"}
		for _, f := range fixedFields {
			if displayName, ok := fieldMap[f]; ok {
				fields = append(fields, StatsField{Name: f, DisplayName: displayName})
			} else {
				fields = append(fields, StatsField{Name: f, DisplayName: f})
			}
		}
	} else {
		for fieldName, displayName := range fieldMap {
			fields = append(fields, StatsField{Name: fieldName, DisplayName: displayName})
		}
	}

	return fields
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
