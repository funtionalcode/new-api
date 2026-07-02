package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func setupUserManageControllerTestDB(t *testing.T) {
	t.Helper()

	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}))
}

func TestManageUserRestoreClearsSoftDelete(t *testing.T) {
	setupUserManageControllerTestDB(t)

	user := &model.User{
		Username: "restore_me",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, model.DB.Create(user).Error)
	require.NoError(t, model.DB.Delete(user).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/manage", map[string]any{
		"id":     user.Id,
		"action": "restore",
	}, 1)
	ctx.Set("role", common.RoleRootUser)
	ctx.Set("username", "root")

	ManageUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var restored model.User
	require.NoError(t, model.DB.First(&restored, user.Id).Error)
	require.False(t, restored.DeletedAt.Valid)
}
