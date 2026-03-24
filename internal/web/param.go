package web

type Param string

const (
	MediaIDParam          Param = "MediaID"
	PublishStateParam     Param = "PublishState"
	LangParam             Param = "Lang"
	OtherUserIDParam      Param = "OtherUserID"
	UserIDParam           Param = "UserID"
	ModerationIDParam     Param = "ModerationID"
	MediaModerationAction Param = "MediaModerationAction"
)

func (p Param) Path() string {
	return ":" + string(p)
}
