package service

import (
	"a1in-bot/app/mikanrss/internal/event"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	qqbot "qqbot/api"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
)

type MikanRSSService interface {
	Start(context.Context) error
	Stop(context.Context) error
}

type MikanRSS struct {
	ctx     context.Context
	conn    *websocket.Conn
	proxy   qqbot.ProxyClient
	handler []event.EventHandler
}

func NewMikanRSSService() MikanRSSService {
	return &MikanRSS{
		handler: make([]event.EventHandler, 0),
	}
}

func (s *MikanRSS) Start(ctx context.Context) (err error) {
	addr := "127.0.0.1:7999"
	origin := fmt.Sprintf("http://%v", addr)
	u := url.URL{Scheme: "ws", Host: addr, Path: "/notify"}
	header := make(http.Header)
	header.Add("Origin", origin)

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		log.Context(ctx).Errorf("[mikan] dial ws://%v/notify err: %v", addr, err)
		return
	}

	pConn, err := grpc.Dial("127.0.0.1:9000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Context(ctx).Errorf("[mikan] dial grpc://127.0.0.1:9000 err: %v", addr, err)
		return
	}

	s.ctx = ctx
	s.conn = conn
	s.proxy = qqbot.NewProxyClient(pConn)
	s.handler = append(s.handler, event.NewMikanRSSEventHandler(s.proxy))

	go s.handle()
	return
}

func (s *MikanRSS) Stop(ctx context.Context) (err error) {
	return s.conn.Close()
}

func (s *MikanRSS) handle() {
	req := &qqbot.RegisterReq{
		Name:  "MikanRSS",
		Token: "",
		NeedEventList: []qqbot.EventId{
			qqbot.EventId_MessageEventAll,
		},
	}
	reqData, _ := proto.Marshal(req)
	s.conn.WriteMessage(websocket.BinaryMessage, reqData)
	_, respData, err := s.conn.ReadMessage()
	if err != nil {
		s.proxy.SendDebugMsg(s.ctx, &qqbot.SendDebugMsgReq{Message: []*qqbot.Segment{qqbot.BuildTextSegment(fmt.Sprintf("MikanRSS 写注册消息失败：%v", err))}})
		log.Context(s.ctx).Errorf("[mikan] read register resp err: %v", err)
		return
	}
	resp := &qqbot.RegisterResp{}
	err = proto.Unmarshal(respData, resp)
	if err != nil {
		s.proxy.SendDebugMsg(s.ctx, &qqbot.SendDebugMsgReq{Message: []*qqbot.Segment{qqbot.BuildTextSegment(fmt.Sprintf("MikanRSS 读注册消息失败：%v", err))}})
		log.Context(s.ctx).Errorf("[mikan] unmarshal register resp err: %v", err)
		return
	}
	log.Context(s.ctx).Infof("[mikan] register resp: %v", resp)
	for {
		_, eventData, err := s.conn.ReadMessage()
		if err != nil {
			s.proxy.SendDebugMsg(s.ctx, &qqbot.SendDebugMsgReq{Message: []*qqbot.Segment{qqbot.BuildTextSegment(fmt.Sprintf("MikanRSS 获取事件通知消息失败：%v", err))}})
			log.Context(s.ctx).Errorf("[mikan] read event message err: %v", err)
			break
		}
		event := &qqbot.NotifyEvent{}
		err = proto.Unmarshal(eventData, event)
		if err != nil {
			s.proxy.SendDebugMsg(s.ctx, &qqbot.SendDebugMsgReq{Message: []*qqbot.Segment{qqbot.BuildTextSegment(fmt.Sprintf("MikanRSS 解析事件通知消息失败：%v", err))}})
			log.Context(s.ctx).Errorf("[mikan] unmarshal event message err: %v", err)
			break
		}
		for _, h := range s.handler {
			if h.Match(event) {
				h.Handle(event)
			}
		}
	}
	os.Exit(0)
}
