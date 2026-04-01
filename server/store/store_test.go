package store

import (
	"encoding/json"
	"testing"

	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-confluence/server/config"
	"github.com/mattermost/mattermost-plugin-confluence/server/util/types"
)

func TestStoreConnectionStoresUsernameLookupWhenDifferent(t *testing.T) {
	mockAPI := &plugintest.API{}
	config.Mattermost = mockAPI

	connection := &types.Connection{
		ConfluenceUser: types.ConfluenceUser{
			AccountID:   "user-key-1",
			Name:        "alice",
			DisplayName: "Alice",
		},
	}

	mockAPI.On("KVSet", "https://conf.example.com_mm-user-1", mock.AnythingOfType("[]uint8")).Return(nil).Once()
	mockAPI.On("KVSet", "https://conf.example.com_user-key-1", mock.AnythingOfType("[]uint8")).Return(nil).Once()
	mockAPI.On("KVSet", "https://conf.example.com_alice", mock.AnythingOfType("[]uint8")).Return(nil).Once()
	mockAPI.On("LogDebug", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Once()

	err := StoreConnection("https://conf.example.com", "mm-user-1", connection)
	require.NoError(t, err)
	mockAPI.AssertExpectations(t)
}

func TestDeleteConnectionDeletesUsernameLookupWhenDifferent(t *testing.T) {
	mockAPI := &plugintest.API{}
	config.Mattermost = mockAPI

	connection := &types.Connection{
		ConfluenceUser: types.ConfluenceUser{
			AccountID:   "user-key-1",
			Name:        "alice",
			DisplayName: "Alice",
		},
	}

	connectionBytes, err := json.Marshal(connection)
	require.NoError(t, err)

	mockAPI.On("KVGet", "https://conf.example.com_mm-user-1").Return(connectionBytes, nil).Once()
	mockAPI.On("KVDelete", "https://conf.example.com_mm-user-1").Return(nil).Once()
	mockAPI.On("KVDelete", "https://conf.example.com_user-key-1").Return(nil).Once()
	mockAPI.On("KVDelete", "https://conf.example.com_alice").Return(nil).Once()
	mockAPI.On("LogDebug", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Once()

	err = DeleteConnection("https://conf.example.com", "mm-user-1")
	require.NoError(t, err)
	mockAPI.AssertExpectations(t)
}
