package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/store"
	"github.com/mattermost/mattermost-plugin-confluence/server/util/types"
)

const (
	settingOn  = "on"
	settingOff = "off"

	settingMentionRole  = "mention"
	settingWatchingRole = "watching"
)

func ensureConnectionSettings(connection *types.Connection) {
	if connection.Settings == nil {
		connection.Settings = &types.ConnectionSettings{
			Notifications: true,
		}
	}

	if connection.Settings.RolesForDMNotification == nil {
		connection.Settings.RolesForDMNotification = map[string]bool{}
	}
}

func updateNotificationSetting(connection *types.Connection, role, value string) (string, bool) {
	switch role {
	case settingMentionRole, settingWatchingRole:
	default:
		return fmt.Sprintf("* Invalid role `%s`. Accepted roles are: `mention` or `watching`.", role), false
	}

	var enabled bool
	switch value {
	case settingOn:
		enabled = true
	case settingOff:
		enabled = false
	default:
		return fmt.Sprintf("* Invalid value `%s`. Accepted values are: `on` or `off`.", value), false
	}

	ensureConnectionSettings(connection)
	connection.Settings.RolesForDMNotification[role] = enabled

	return "", true
}

func updateGlobalNotificationSetting(connection *types.Connection, value string) (string, bool) {
	var enabled bool
	switch value {
	case settingOn:
		enabled = true
	case settingOff:
		enabled = false
	default:
		return fmt.Sprintf("* Invalid value `%s`. Accepted values are: `on` or `off`.", value), false
	}

	ensureConnectionSettings(connection)
	connection.Settings.Notifications = enabled

	return "", true
}

func getNotificationSettingsText(connection *types.Connection) string {
	ensureConnectionSettings(connection)

	globalState := settingOff
	if connection.Settings.Notifications {
		globalState = settingOn
	}

	mentionState := settingOff
	if connection.ShouldReceiveNotification(settingMentionRole) {
		mentionState = settingOn
	}

	watchingState := settingOff
	if connection.ShouldReceiveNotification(settingWatchingRole) {
		watchingState = settingOn
	}

	return fmt.Sprintf("Current settings:\n\t- Notifications: %s \n\t- Notifications for mention: %s \n\t- Notifications for watching: %s", globalState, mentionState, watchingState)
}

func settingsNotificationsCommand(p *Plugin, header *model.CommandArgs, args ...string) *model.CommandResponse {
	pluginConfig := config.GetConfig()
	if !pluginConfig.ServerVersionGreaterthan9 {
		return p.responsef(header, "Personal notification settings are only available for Confluence Server/DC >= 9.")
	}

	connection, err := store.LoadConnection(pluginConfig.ConfluenceURL, header.UserId)
	if err != nil {
		return p.responsef(header, "Failed to load your Confluence connection. Error: %v", err)
	}

	ensureConnectionSettings(connection)

	if len(args) == 0 {
		return p.responsef(header, "%s", getNotificationSettingsText(connection))
	}

	if len(args) == 1 {
		helpTextSuffix, ok := updateGlobalNotificationSetting(connection, args[0])
		if !ok {
			return p.responsef(header, "`/confluence settings notifications [on|off]`\n`/confluence settings notifications [mention|watching] [on|off]`\n%s", helpTextSuffix)
		}

		if err := store.StoreConnection(pluginConfig.ConfluenceURL, header.UserId, connection); err != nil {
			return p.responsef(header, "Could not store new settings. Please contact your system administrator. Error: %v", err)
		}

		state := settingOff
		if connection.Settings.Notifications {
			state = settingOn
		}

		return p.responsef(header, "Settings updated:\n* Notifications %s.", state)
	}

	if len(args) != 2 {
		return p.responsef(header, "`/confluence settings notifications [on|off]`\n`/confluence settings notifications [mention|watching] [on|off]`\n* Invalid command args.")
	}

	helpTextSuffix, ok := updateNotificationSetting(connection, args[0], args[1])
	if !ok {
		return p.responsef(header, "`/confluence settings notifications [on|off]`\n`/confluence settings notifications [mention|watching] [on|off]`\n%s", helpTextSuffix)
	}

	if err := store.StoreConnection(pluginConfig.ConfluenceURL, header.UserId, connection); err != nil {
		return p.responsef(header, "Could not store new settings. Please contact your system administrator. Error: %v", err)
	}

	state := settingOff
	if connection.ShouldReceiveNotification(args[0]) {
		state = settingOn
	}

	return p.responsef(header, "Settings updated:\n* %s notifications %s.", capitalize(args[0]), state)
}

func capitalize(value string) string {
	if value == "" {
		return value
	}

	return strings.ToUpper(value[:1]) + value[1:]
}
