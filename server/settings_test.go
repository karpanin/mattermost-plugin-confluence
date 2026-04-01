package main

import (
	"encoding/json"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/util/types"
)

func TestUpdateNotificationSetting(t *testing.T) {
	connection := &types.Connection{}

	msg, ok := updateNotificationSetting(connection, settingMentionRole, settingOff)
	require.True(t, ok)
	assert.Empty(t, msg)
	assert.False(t, connection.ShouldReceiveNotification(settingMentionRole))

	msg, ok = updateNotificationSetting(connection, "invalid", settingOn)
	require.False(t, ok)
	assert.Contains(t, msg, "Invalid role")
}

func TestUpdateGlobalNotificationSetting(t *testing.T) {
	connection := &types.Connection{}

	msg, ok := updateGlobalNotificationSetting(connection, settingOff)
	require.True(t, ok)
	assert.Empty(t, msg)
	assert.False(t, connection.Settings.Notifications)

	msg, ok = updateGlobalNotificationSetting(connection, "invalid")
	require.False(t, ok)
	assert.Contains(t, msg, "Invalid value")
}

func TestGetNotificationSettingsText(t *testing.T) {
	connection := &types.Connection{
		Settings: &types.ConnectionSettings{
			Notifications: true,
			RolesForDMNotification: map[string]bool{
				settingMentionRole:  true,
				settingWatchingRole: false,
			},
		},
	}

	text := getNotificationSettingsText(connection)
	assert.Contains(t, text, "Notifications: on")
	assert.Contains(t, text, "Notifications for mention: on")
	assert.Contains(t, text, "Notifications for watching: off")
}

func TestSettingsNotificationsCommand(t *testing.T) {
	mockAPI := &plugintest.API{}
	config.Mattermost = mockAPI
	config.SetConfig(&config.Configuration{
		ConfluenceURL:             "https://conf.example.com",
		ServerVersionGreaterthan9: true,
	})

	connection := &types.Connection{
		ConfluenceUser: types.ConfluenceUser{AccountID: "acc-1", Name: "demo"},
		Settings: &types.ConnectionSettings{
			Notifications: true,
			RolesForDMNotification: map[string]bool{
				settingMentionRole:  true,
				settingWatchingRole: true,
			},
		},
	}
	connectionBytes, err := json.Marshal(connection)
	require.NoError(t, err)

	mockAPI.On("KVGet", "https://conf.example.com_user-1").Return(connectionBytes, nil).Once()
	mockAPI.On("KVSet", "https://conf.example.com_user-1", mock.AnythingOfType("[]uint8")).Return(nil).Once()
	mockAPI.On("KVSet", "https://conf.example.com_acc-1", mock.AnythingOfType("[]uint8")).Return(nil).Once()
	mockAPI.On("KVSet", "https://conf.example.com_demo", mock.AnythingOfType("[]uint8")).Return(nil).Once()
	mockAPI.On("LogDebug", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Once()
	mockAPI.On("SendEphemeralPost", "user-1", mock.AnythingOfType("*model.Post")).Run(func(args mock.Arguments) {
		post := args.Get(1).(*model.Post)
		assert.Contains(t, post.Message, "Settings updated")
		assert.Contains(t, post.Message, "Mention notifications off")
	}).Return(&model.Post{}).Once()

	p := &Plugin{}
	resp := settingsNotificationsCommand(p, &model.CommandArgs{UserId: "user-1", ChannelId: "chan-1"}, settingMentionRole, settingOff)
	require.NotNil(t, resp)
	mockAPI.AssertExpectations(t)
}

func TestSettingsNotificationsCommandGlobalToggle(t *testing.T) {
	mockAPI := &plugintest.API{}
	config.Mattermost = mockAPI
	config.SetConfig(&config.Configuration{
		ConfluenceURL:             "https://conf.example.com",
		ServerVersionGreaterthan9: true,
	})

	connection := &types.Connection{
		ConfluenceUser: types.ConfluenceUser{AccountID: "acc-1", Name: "demo"},
		Settings: &types.ConnectionSettings{
			Notifications: true,
			RolesForDMNotification: map[string]bool{
				settingMentionRole:  true,
				settingWatchingRole: true,
			},
		},
	}
	connectionBytes, err := json.Marshal(connection)
	require.NoError(t, err)

	mockAPI.On("KVGet", "https://conf.example.com_user-1").Return(connectionBytes, nil).Once()
	mockAPI.On("KVSet", "https://conf.example.com_user-1", mock.AnythingOfType("[]uint8")).Return(nil).Once()
	mockAPI.On("KVSet", "https://conf.example.com_acc-1", mock.AnythingOfType("[]uint8")).Return(nil).Once()
	mockAPI.On("KVSet", "https://conf.example.com_demo", mock.AnythingOfType("[]uint8")).Return(nil).Once()
	mockAPI.On("LogDebug", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Once()
	mockAPI.On("SendEphemeralPost", "user-1", mock.AnythingOfType("*model.Post")).Run(func(args mock.Arguments) {
		post := args.Get(1).(*model.Post)
		assert.Contains(t, post.Message, "Settings updated")
		assert.Contains(t, post.Message, "Notifications off")
	}).Return(&model.Post{}).Once()

	p := &Plugin{}
	resp := settingsNotificationsCommand(p, &model.CommandArgs{UserId: "user-1", ChannelId: "chan-1"}, settingOff)
	require.NotNil(t, resp)
	mockAPI.AssertExpectations(t)
}
