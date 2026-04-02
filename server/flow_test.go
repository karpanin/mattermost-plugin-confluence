package main

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
)

func TestStartCompletionWizardSkipsNonSystemAdmin(t *testing.T) {
	mockAPI := baseMock()
	mockAPI.On("GetUser", "user-id").Return(&model.User{Id: "user-id", Roles: model.SystemUserRoleId}, nil).Once()

	fm := &FlowManager{}

	err := fm.StartCompletionWizard("user-id")
	require.NoError(t, err)

	mockAPI.AssertExpectations(t)
}
