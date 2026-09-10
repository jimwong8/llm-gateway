package i18n

import "strings"

type Catalog map[string]map[string]string

var catalog = Catalog{
	"invalid_request_error": {
		"zh-CN": "无效请求",
		"en-US": "invalid request",
	},
	"missing_required_fields": {
		"zh-CN": "缺少必填字段",
		"en-US": "missing required fields",
	},
	"resource_not_found": {
		"zh-CN": "资源未找到",
		"en-US": "resource not found",
	},
	"method_not_allowed": {
		"zh-CN": "请求方法不允许",
		"en-US": "method not allowed",
	},
	"internal_server_error": {
		"zh-CN": "服务器内部错误",
		"en-US": "internal server error",
	},
	"authentication_required": {
		"zh-CN": "需要身份验证",
		"en-US": "admin authentication required",
	},
	"unauthorized": {
		"zh-CN": "未授权",
		"en-US": "unauthorized",
	},
	"authorization_error": {
		"zh-CN": "权限不足",
		"en-US": "role not permitted for admin endpoint",
	},
	"invalid_json_body": {
		"zh-CN": "无效的 JSON 请求体",
		"en-US": "invalid JSON body",
	},
	"admin_store_unavailable": {
		"zh-CN": "管理存储不可用",
		"en-US": "admin store unavailable",
	},
	"billing_store_unavailable": {
		"zh-CN": "计费存储不可用",
		"en-US": "billing store unavailable",
	},
	"quota_limiter_unavailable": {
		"zh-CN": "配额限制器不可用",
		"en-US": "quota limiter unavailable",
	},
	"tenant_key_store_unavailable": {
		"zh-CN": "租户密钥存储不可用",
		"en-US": "tenant key store unavailable",
	},
	"invalid_asset_id": {
		"zh-CN": "无效的资产 ID",
		"en-US": "invalid asset id",
	},
	"asset_not_found": {
		"zh-CN": "资产未找到",
		"en-US": "asset not found",
	},
	"asset_id_required": {
		"zh-CN": "需要资产 ID",
		"en-US": "asset id required",
	},
	"channel_id_required": {
		"zh-CN": "需要渠道 ID",
		"en-US": "channel id required",
	},
	"name_and_provider_required": {
		"zh-CN": "名称和提供商为必填项",
		"en-US": "name and provider are required",
	},
	"tenant_id_model_required": {
		"zh-CN": "租户 ID 和模型为必填项",
		"en-US": "tenant_id and model are required",
	},
	"snapshot_not_found": {
		"zh-CN": "快照未找到",
		"en-US": "snapshot not found",
	},
	"no_snapshots_to_import": {
		"zh-CN": "没有可导入的快照",
		"en-US": "no snapshots to import",
	},
	"source_and_target_must_differ": {
		"zh-CN": "源环境和目标环境不能相同",
		"en-US": "source and target environment must differ",
	},
	"runtime_replay_unavailable": {
		"zh-CN": "运行时重放不可用",
		"en-US": "runtime replay unavailable",
	},
	"version_not_found": {
		"zh-CN": "版本未找到",
		"en-US": "version not found",
	},
	"version_is_not_released": {
		"zh-CN": "版本尚未发布",
		"en-US": "version is not released",
	},
	"version_target_ambiguous": {
		"zh-CN": "版本目标不明确，请指定范围和项目 ID",
		"en-US": "version target is ambiguous; specify scope and project_id",
	},
	"version_id_required": {
		"zh-CN": "需要版本 ID",
		"en-US": "version id required",
	},
	"invalid_version_id": {
		"zh-CN": "无效的版本 ID",
		"en-US": "invalid version id",
	},
	"unknown_action": {
		"zh-CN": "未知操作",
		"en-US": "unknown action",
	},
	"invalid_path": {
		"zh-CN": "无效路径",
		"en-US": "invalid path",
	},
	"route_not_found": {
		"zh-CN": "路由未找到",
		"en-US": "route not found",
	},
	"request_timeout": {
		"zh-CN": "请求超时",
		"en-US": "Request timeout",
	},
	"network_error": {
		"zh-CN": "网络错误，请检查网络连接",
		"en-US": "Network error - please check your connection",
	},
	"token_format_invalid": {
		"zh-CN": "Token 格式无效，长度至少 4 个字符",
		"en-US": "Token format invalid, minimum 4 characters",
	},
	"login_failed": {
		"zh-CN": "登录失败",
		"en-US": "login failed",
	},
	"email_and_password_required": {
		"zh-CN": "请填写邮箱和密码",
		"en-US": "email and password are required",
	},
	"token_required": {
		"zh-CN": "请输入管理员 Token",
		"en-US": "admin token is required",
	},
}

func T(lang, key string) string {
	if entry, ok := catalog[key]; ok {
		if msg, ok := entry[lang]; ok {
			return msg
		}
		if msg, ok := entry["en-US"]; ok {
			return msg
		}
	}
	return key
}

func AcceptLanguage(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return "zh-CN"
	}
	parts := strings.Split(header, ",")
	for _, part := range parts {
		lang := strings.TrimSpace(part)
		if idx := strings.Index(lang, ";"); idx != -1 {
			lang = strings.TrimSpace(lang[:idx])
		}
		lower := strings.ToLower(lang)
		if strings.HasPrefix(lower, "zh") {
			return "zh-CN"
		}
		if strings.HasPrefix(lower, "en") {
			return "en-US"
		}
	}
	return "zh-CN"
}
