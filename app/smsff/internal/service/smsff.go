package service

import (
	"a1in-bot/app/smsff/api"
	"context"
	qqbot "qqbot/api"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// SmsFFService is a greeter service.
type SmsFFService struct {
	api.UnimplementedSmsFFServer

	proxy qqbot.ProxyClient
}

func NewSmsFFService() *SmsFFService {
	conn, err := grpc.Dial("127.0.0.1:9000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	return &SmsFFService{
		proxy: qqbot.NewProxyClient(conn),
	}
}

func (s *SmsFFService) SendSms(ctx context.Context, req *api.SendSmsReq) (resp *api.SendSmsResp, err error) {
	resp = &api.SendSmsResp{}
	defer log.Context(ctx).Infof("[SmsFFService] req: %v, resp: %v, err: %v")
	_, err = s.proxy.SendGroupMsg(ctx, &qqbot.SendGroupMsgReq{
		GroupId:    253016320,
		Message:    []*qqbot.Segment{qqbot.BuildTextSegment(req.GetText())},
		AutoEscape: false,
	})
	return
}
