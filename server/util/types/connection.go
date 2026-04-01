package types

type User struct {
	MattermostUserID string `json:"mattermost_user_id"`
	InstanceURL      string `json:"instance_url,omitempty"`
}

type ConfluenceUser struct {
	AccountID   string `json:"accountId,omitempty"`
	Name        string `json:"username,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

type UserGroups struct {
	Groups []*UserGroup `json:"results,omitempty"`
}

type UserGroup struct {
	Name string `json:"name"`
}

type Connection struct {
	ConfluenceUser
	OAuth2Token       string `json:"token,omitempty"`
	DefaultProjectKey string `json:"default_project_key,omitempty"`
	IsAdmin           bool   `json:"is_admin,omitempty"`
	MattermostUserID  string `json:"mattermost_user_id,omitempty"`
	Settings          *ConnectionSettings `json:"settings,omitempty"`
}

type ConnectionSettings struct {
	Notifications          bool            `json:"notifications"`
	RolesForDMNotification map[string]bool `json:"roles_for_dm_notification,omitempty"`
	LastSelectedSpaceKey   string          `json:"last_selected_space_key,omitempty"`
}

func (c *Connection) ConfluenceAccountID() string {
	if c.AccountID != "" {
		return c.AccountID
	}

	return c.Name
}

func NewUser(mattermostUserID string) *User {
	return &User{
		MattermostUserID: mattermostUserID,
	}
}

func (c *Connection) ShouldReceiveNotification(role string) bool {
	if c.Settings == nil {
		return true
	}

	if !c.Settings.Notifications {
		return false
	}

	if c.Settings.RolesForDMNotification == nil {
		return true
	}

	value, ok := c.Settings.RolesForDMNotification[role]
	if !ok {
		return true
	}

	return value
}

func (user *User) AsConfigMap() map[string]interface{} {
	return map[string]interface{}{
		"mattermost_user_id": user.MattermostUserID,
		"instance_url":       user.InstanceURL,
	}
}
