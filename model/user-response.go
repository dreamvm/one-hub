package model

// AdminUserResponse is the explicit field allowlist for cross-user management
// responses. Never embed User here: new authentication fields must stay private.
type AdminUserResponse struct {
	Id              int    `json:"id"`
	Username        string `json:"username"`
	Password        string `json:"password"` // Always empty; retained for the edit form.
	DisplayName     string `json:"display_name"`
	Role            int    `json:"role"`
	Status          int    `json:"status"`
	Email           string `json:"email"`
	AvatarUrl       string `json:"avatar_url"`
	OidcId          string `json:"oidc_id"`
	GitHubId        string `json:"github_id"`
	GitHubIdNew     int    `json:"github_id_new"`
	WeChatId        string `json:"wechat_id"`
	TelegramId      int64  `json:"telegram_id"`
	LarkId          string `json:"lark_id"`
	Quota           int    `json:"quota"`
	UsedQuota       int    `json:"used_quota"`
	RequestCount    int    `json:"request_count"`
	Group           string `json:"group"`
	AffCode         string `json:"aff_code"`
	AffCount        int    `json:"aff_count"`
	AffQuota        int    `json:"aff_quota"`
	AffHistoryQuota int    `json:"aff_history_quota"`
	InviterId       int    `json:"inviter_id"`
	LastLoginTime   int64  `json:"last_login_time"`
	LastLoginIp     string `json:"last_login_ip"`
	CreatedTime     int64  `json:"created_time"`
}

func (user *User) AdminResponse() *AdminUserResponse {
	return &AdminUserResponse{
		Id: user.Id, Username: user.Username, DisplayName: user.DisplayName,
		Role: user.Role, Status: user.Status, Email: user.Email, AvatarUrl: user.AvatarUrl,
		OidcId: user.OidcId, GitHubId: user.GitHubId, GitHubIdNew: user.GitHubIdNew,
		WeChatId: user.WeChatId, TelegramId: user.TelegramId, LarkId: user.LarkId,
		Quota: user.Quota, UsedQuota: user.UsedQuota, RequestCount: user.RequestCount, Group: user.Group,
		AffCode: user.AffCode, AffCount: user.AffCount, AffQuota: user.AffQuota,
		AffHistoryQuota: user.AffHistoryQuota, InviterId: user.InviterId,
		LastLoginTime: user.LastLoginTime, LastLoginIp: user.LastLoginIp, CreatedTime: user.CreatedTime,
	}
}
