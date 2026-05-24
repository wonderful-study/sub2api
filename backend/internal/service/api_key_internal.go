package service

import "strconv"

const (
	onlineExperienceInternalAPIKeyNamePrefix = "__online_experience__:"
	onlineExperienceVisibleAPIKeyName        = "在线体验"
)

func OnlineExperienceInternalAPIKeyNamePrefix() string {
	return onlineExperienceInternalAPIKeyNamePrefix
}

func BuildOnlineExperienceInternalAPIKeyName(groupID int64) string {
	return onlineExperienceInternalAPIKeyNamePrefix + strconv.FormatInt(groupID, 10)
}

func IsInternalAPIKeyName(name string) bool {
	return len(name) > len(onlineExperienceInternalAPIKeyNamePrefix) && name[:len(onlineExperienceInternalAPIKeyNamePrefix)] == onlineExperienceInternalAPIKeyNamePrefix
}

func VisibleAPIKeyName(name string) string {
	if IsInternalAPIKeyName(name) {
		return onlineExperienceVisibleAPIKeyName
	}
	return name
}

func IsVisibleAPIKeyName(name string) bool {
	return !IsInternalAPIKeyName(name)
}
