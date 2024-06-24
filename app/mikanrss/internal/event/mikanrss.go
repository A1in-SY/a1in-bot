package event

import (
	"a1in-bot/app/mikanrss/internal/model"
	"a1in-bot/app/mikanrss/internal/repo"
	"context"
	"encoding/json"
	"fmt"
	"os"
	qqbot "qqbot/api"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

type MikanRSSEventHandler struct {
	proxy         qqbot.ProxyClient
	mikan         *repo.MikanClient
	user_rss_map  map[int64]UserRSS
	rss_mu        sync.RWMutex
	user_read_map map[int64]map[string]struct{}
	read_mu       sync.RWMutex
}

type UserRSS struct {
	groupIds map[int64]struct{}
	url      string
}

func NewMikanRSSEventHandler(proxy qqbot.ProxyClient) *MikanRSSEventHandler {
	m1 := make(map[int64]UserRSS)
	m2 := make(map[int64]map[string]struct{})
	if _, err := os.Stat("user_rss_map.json"); os.IsNotExist(err) {
		d1, _ := json.Marshal(m1)
		d2, _ := json.Marshal(m2)
		err := os.WriteFile("user_rss_map.json", d1, 0644)
		if err != nil {
			panic(err)
		}
		err = os.WriteFile("user_read_map.json", d2, 0644)
		if err != nil {
			panic(err)
		}
	} else {
		d1, err := os.ReadFile("user_rss_map.json")
		if err != nil {
			panic(err)
		}
		err = json.Unmarshal(d1, &m1)
		if err != nil {
			panic(err)
		}
		d2, err := os.ReadFile("user_read_map.json")
		if err != nil {
			panic(err)
		}
		err = json.Unmarshal(d2, &m2)
		if err != nil {
			panic(err)
		}
	}
	h := &MikanRSSEventHandler{
		mikan:         repo.NewMikanClient(),
		proxy:         proxy,
		user_rss_map:  m1,
		user_read_map: m2,
	}
	go h.loop()
	return h
}

func (h *MikanRSSEventHandler) Match(event *qqbot.NotifyEvent) (isMatch bool) {
	// defer log.Infof("[mikan] event: %v, isMatch: %v", event, isMatch)
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
	if strings.Split(strings.TrimLeft(eventData.GroupMsg.GetMessage()[1].Data.Text, " "), " ")[0] != "mikan" {
		return
	}
	if len(strings.Split(strings.TrimLeft(eventData.GroupMsg.GetMessage()[1].Data.Text, " "), " ")) != 3 {
		return
	}
	isMatch = true
	return
}

// mikan bind url
// mikan unbind url
func (h *MikanRSSEventHandler) Handle(event *qqbot.NotifyEvent) {
	eventData := event.NotifyEventData.(*qqbot.NotifyEvent_GroupMsg)
	cmd := strings.Split(strings.TrimLeft(eventData.GroupMsg.GetMessage()[1].Data.Text, " "), " ")
	userId := eventData.GroupMsg.GetUserId()
	groupId := eventData.GroupMsg.GetGroupId()
	operation := cmd[1]
	rssUrl := cmd[2]
	if operation == "bind" {
		// 先验证这个链接是不是有效，无效直接返回
		rssFeed, err := h.mikan.GetRSSFeed(rssUrl)
		if err != nil {
			log.Errorf("[mikan] operation bind, get rss feed err: %v", err)
			h.proxy.SendGroupMsg(context.Background(), &qqbot.SendGroupMsgReq{
				GroupId: groupId,
				Message: []*qqbot.Segment{qqbot.BuildAtSegment(strconv.FormatInt(userId, 10)), qqbot.BuildTextSegment(fmt.Sprintf(" 从指定链接获取数据失败：%v", err))},
			})
			return
		}

		// 有效后再检查以前有没有绑定过
		h.rss_mu.Lock()
		if oldRss, ok := h.user_rss_map[userId]; ok {
			log.Infof("[mikan] operation bind, user %v has bind rss url %v", userId, oldRss.url)
			h.proxy.SendGroupMsg(context.Background(), &qqbot.SendGroupMsgReq{
				GroupId: groupId,
				Message: []*qqbot.Segment{qqbot.BuildAtSegment(strconv.FormatInt(userId, 10)), qqbot.BuildTextSegment(fmt.Sprintf(" 之前已绑定过 Mikan RSS 链接：%v，已更换为指定链接", oldRss.url))},
			})
			oldRss.groupIds[groupId] = struct{}{}
			h.user_rss_map[userId] = UserRSS{
				groupIds: oldRss.groupIds,
				url:      rssUrl,
			}
		} else {
			log.Infof("[mikan] operation bind, user %v bind rss url %v", userId, rssUrl)
			h.proxy.SendGroupMsg(context.Background(), &qqbot.SendGroupMsgReq{
				GroupId: groupId,
				Message: []*qqbot.Segment{qqbot.BuildAtSegment(strconv.FormatInt(userId, 10)), qqbot.BuildTextSegment(" 成功绑定 Mikan RSS 链接")},
			})
			h.user_rss_map[userId] = UserRSS{
				groupIds: map[int64]struct{}{
					groupId: {},
				},
				url: rssUrl,
			}
		}
		d1, _ := json.Marshal(h.user_rss_map)
		err = os.WriteFile("user_rss_map.json", d1, 0644)
		if err != nil {
			log.Errorf("[mikan] operation bind, write user_rss_map.json err: %v", err)
			h.proxy.SendGroupMsg(context.Background(), &qqbot.SendGroupMsgReq{
				GroupId: groupId,
				Message: []*qqbot.Segment{qqbot.BuildAtSegment(strconv.FormatInt(userId, 10)), qqbot.BuildTextSegment(" 持久化数据时失败，叫开发出来挨打")},
			})
		}
		h.rss_mu.Unlock()

		h.read_mu.Lock()
		_, ok := h.user_read_map[userId]
		if !ok {
			h.user_read_map[userId] = make(map[string]struct{})
		}
		text := " 你的 Mikan RSS 源现在有以下内容\n\n"
		for _, item := range rssFeed.Channel.Items {
			text += fmt.Sprintf("标题：%v\nMikan 链接：%v\n种子地址：%v\n\n", item.Description, item.Link, item.Enclosure.URL)
			h.user_read_map[userId][item.Link] = struct{}{}
		}
		d2, _ := json.Marshal(h.user_read_map)
		err = os.WriteFile("user_read_map.json", d2, 0644)
		if err != nil {
			log.Errorf("[mikan] operation bind, write user_read_map.json err: %v", err)
			h.proxy.SendGroupMsg(context.Background(), &qqbot.SendGroupMsgReq{
				GroupId: groupId,
				Message: []*qqbot.Segment{qqbot.BuildAtSegment(strconv.FormatInt(userId, 10)), qqbot.BuildTextSegment(" 持久化数据时失败，叫开发出来挨打")},
			})
		}
		h.read_mu.Unlock()

		h.proxy.SendGroupMsg(context.Background(), &qqbot.SendGroupMsgReq{
			GroupId: groupId,
			Message: []*qqbot.Segment{qqbot.BuildAtSegment(strconv.FormatInt(userId, 10)), qqbot.BuildTextSegment(text)},
		})
	}
}

func (h *MikanRSSEventHandler) loop() {
	for {
		h.rss_mu.RLock()
		h.read_mu.Lock()
		for userId, userRSS := range h.user_rss_map {
			rssFeed, err := h.mikan.GetRSSFeed(userRSS.url)
			if err != nil {
				log.Errorf("[mikan] loop, get rss feed err: %v", err)
				h.proxy.SendDebugMsg(context.Background(), &qqbot.SendDebugMsgReq{
					Message: []*qqbot.Segment{qqbot.BuildAtSegment(strconv.FormatInt(userId, 10)), qqbot.BuildTextSegment(fmt.Sprintf(" 从指定链接更新数据失败：%v", err))},
				})
				continue
			}
			_, ok := h.user_read_map[userId]
			if !ok {
				h.user_read_map[userId] = make(map[string]struct{})
			}
			text := " 检测到你的 Mikan RSS 源有以下更新\n\n"
			for _, item := range rssFeed.Channel.Items {
				if _, ok := h.user_read_map[userId][item.Link]; !ok {
					text += fmt.Sprintf("标题：%v\nMikan 链接：%v\n种子地址：%v\n\n", item.Description, item.Link, item.Enclosure.URL)
					h.user_read_map[userId][item.Link] = struct{}{}
				}
			}
			for groupId := range userRSS.groupIds {
				h.proxy.SendGroupMsg(context.Background(), &qqbot.SendGroupMsgReq{
					GroupId: groupId,
					Message: []*qqbot.Segment{qqbot.BuildAtSegment(strconv.FormatInt(userId, 10)), qqbot.BuildTextSegment(text)},
				})
			}
		}
		d1, _ := json.Marshal(h.user_rss_map)
		err1 := os.WriteFile("user_rss_map.json", d1, 0644)
		if err1 != nil {
			log.Errorf("[mikan] loop, write user_rss_map.json err: %v", err1)
		}
		d2, _ := json.Marshal(h.user_read_map)
		err2 := os.WriteFile("user_read_map.json", d2, 0644)
		if err2 != nil {
			log.Errorf("[mikan] loop, write user_rss_map.json err: %v", err2)
		}
		if err1 != nil || err2 != nil {
			h.proxy.SendDebugMsg(context.Background(), &qqbot.SendDebugMsgReq{
				Message: []*qqbot.Segment{qqbot.BuildTextSegment(" Mikan RSS 持久化数据时失败，叫开发出来挨打")},
			})
		}
		h.rss_mu.RUnlock()
		h.read_mu.Unlock()
		time.Sleep(15 * time.Minute)
	}
}
