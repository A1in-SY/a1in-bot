package event

import (
	"a1in-bot/app/mikanrss/internal/model"
	"context"
	"fmt"
	qqbot "qqbot/api"
	"strconv"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
)

type MikanRSSEventHandler struct {
	proxy qqbot.ProxyClient
}

func NewMikanRSSEventHandler(proxy qqbot.ProxyClient) *MikanRSSEventHandler {
	return &MikanRSSEventHandler{
		proxy: proxy,
	}
}

func (h *MikanRSSEventHandler) Match(event *qqbot.NotifyEvent) (isMatch bool) {
	defer log.Infof("[mikan] event: %v, isMatch: %v", event, isMatch)
	isMatch = false
	if event.GetPostType() != model.PostTypeMessage {
		return
	}
	eventData, ok := event.NotifyEventData.(*qqbot.NotifyEvent_GroupMsg)
	if !ok {
		return
	}
	if len(eventData.GroupMsg.GetMessage()) != 2 {
		return
	}
	if eventData.GroupMsg.GetMessage()[0].Type != model.SegmentTypeAt {
		return
	}
	if eventData.GroupMsg.GetMessage()[0].Data.Qq != strconv.FormatInt(event.SelfId, 10) {
		return
	}
	if eventData.GroupMsg.GetMessage()[1].Type != model.SegmentTypeText {
		return
	}
	if !strings.HasPrefix(strings.TrimLeft(eventData.GroupMsg.GetMessage()[1].Data.Text, " "), "mikan") {
		return
	}
	if len(strings.Split(strings.TrimLeft(eventData.GroupMsg.GetMessage()[1].Data.Text, " "), " ")) != 3 {
		return
	}
	isMatch = true
	return
}

func (h *MikanRSSEventHandler) Handle(event *qqbot.NotifyEvent) {
	eventData := event.NotifyEventData.(*qqbot.NotifyEvent_GroupMsg)
	cmd := strings.Split(strings.TrimLeft(eventData.GroupMsg.GetMessage()[1].Data.Text, " "), " ")
	if cmd[1] == "test" {
		h.proxy.SendGroupMsg(context.Background(), &qqbot.SendGroupMsgReq{
			GroupId: eventData.GroupMsg.GroupId,
			Message: []*qqbot.Segment{
				qqbot.BuildAtSegment(strconv.FormatInt(eventData.GroupMsg.UserId, 10)),
				qqbot.BuildTextSegment(fmt.Sprintf(" %v", cmd[2])),
			},
		})
	}
}
