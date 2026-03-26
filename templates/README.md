# Syslog2Bot 内置模板说明

本目录包含 Syslog2Bot 的内置解析模板和筛选策略。

## 文件说明

| 文件 | 说明 |
|------|------|
| parse_templates.json | 日志解析模板（云锁、天眼） |
| filter_policies.json | 筛选策略示例 |

## 使用方法

### 方式一：程序自动加载（推荐）

程序首次运行时会自动加载本目录中的模板文件。

### 方式二：手动导入

1. 打开程序 → 日志解析模板
2. 点击"导入模板"
3. 粘贴 JSON 内容或选择文件

## 模板格式说明

### 解析模板字段

| 字段 | 说明 |
|------|------|
| name | 模板名称 |
| description | 模板描述 |
| parseType | 解析类型（syslog_json/smart_delimiter/regex） |
| headerRegex | 头部正则表达式 |
| fieldMapping | 字段映射 |
| valueTransform | 值转换规则 |
| deviceType | 设备类型 |

### 筛选策略字段

| 字段 | 说明 |
|------|------|
| name | 策略名称 |
| description | 策略描述 |
| parseTemplateName | 关联的解析模板名称 |
| conditions | 筛选条件（JSON数组） |
| conditionLogic | 条件逻辑（AND/OR） |
| action | 动作（keep/drop） |
| priority | 优先级 |
| isActive | 是否启用 |

## 分享模板

你可以将自己创建的模板导出为 JSON 文件，分享给其他用户使用。

## 支持的设备

- 云锁安全设备
- 天眼安全设备

更多设备模板持续更新中...
