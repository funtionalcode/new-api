package model

import "errors"

// GetLogChannelIDsForUser 按当前开放用户名单确定日志范围，不按管理员角色豁免。
func GetLogChannelIDsForUser(userID int) ([]int, error) {
	if userID <= 0 {
		return nil, errors.New("无效的日志查询用户")
	}
	var channels []Channel
	if err := DB.Select("id", "open_user_ids").Find(&channels).Error; err != nil {
		return nil, err
	}
	// 充值等无渠道记录保留可见性，所属用户限制仍由查询接口执行。
	ids := []int{0}
	for _, channel := range channels {
		if channel.IsOpenToUser(userID) {
			ids = append(ids, channel.Id)
		}
	}
	return ids, nil
}
