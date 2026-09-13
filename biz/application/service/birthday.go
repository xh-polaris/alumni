package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/wire"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/config"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/consts"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/birthday"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/mapper/user"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/sms"
	"github.com/xh-polaris/alumni-core_api/biz/infrastructure/util/log"
)

type BirthdayService struct {
	Config         *config.Config
	UserMapper     user.IMongoMapper
	BirthdayMapper birthday.IMongoMapper
	Sender         sms.Sender
}

var BirthdayServiceSet = wire.NewSet(wire.Struct(new(BirthdayService), "*"))

// maxBirthdayAttempts 是单个用户单日最多发送次数：首次一次 + 失败重试两次。
const maxBirthdayAttempts = 3

// ErrBirthdayAttemptsExhausted 表示当天的发送次数已用尽。
var ErrBirthdayAttemptsExhausted = errors.New("当天生日短信发送次数已用尽")

func (s *BirthdayService) Start(ctx context.Context) {
	if !s.Config.BirthdaySms.Enabled || s.Config.BirthdaySms.Endpoint == "" {
		return
	}
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	for {
		now := time.Now().In(location)
		next := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, location)
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			_, _ = s.Run(ctx, next)
		}
	}
}

func (s *BirthdayService) Run(ctx context.Context, date time.Time) (int, error) {
	items, err := s.UserMapper.FindBirthdayCandidates(ctx)
	if err != nil {
		return 0, err
	}
	sent := 0
	failed := 0
	skipped := 0
	for _, item := range items {
		birthdayDate := birthdayOf(item)
		if birthdayDate.IsZero() || !birthdayMatches(birthdayDate, date) {
			continue
		}
		record, err := s.BirthdayMapper.Find(ctx, item.ID.Hex(), date.Year())
		switch {
		case err == nil && record.Status == sms.StatusSent:
			// 同一年度已经发送成功，直接跳过，保证重复执行不会重复发送。
			skipped++
			continue
		case err == nil:
			// 上次失败，保留尝试次数，继续重试。
		case err == consts.ErrNotFound:
			record = &birthday.Log{UserID: item.ID.Hex(), Year: date.Year(), Status: "pending"}
		default:
			return sent, err
		}
		providerID, sendErr := s.sendWithRetry(ctx, item, date.Year(), record)
		if sendErr != nil {
			record.Status, record.LastError = "failed", sendErr.Error()
			failed++
		} else {
			record.Status, record.ProviderID, record.LastError = sms.StatusSent, providerID, ""
			sent++
		}
		if err = s.BirthdayMapper.Save(ctx, record); err != nil {
			return sent, err
		}
	}
	// metric=birthday_sms 供后台监控发送成功率与“重复发送拦截数”（skipped）。
	log.CtxInfo(ctx, "metric=birthday_sms date=%s candidates=%d sent=%d failed=%d duplicate_blocked=%d",
		date.Format("2006-01-02"), len(items), sent, failed, skipped)
	return sent, nil
}

// sendWithRetry 在当天最多尝试 3 次（首次 + 两次重试），并把尝试次数写回记录。
// 如果当天的尝试次数已经用尽，直接返回错误而不是把“未发送”当成“已发送”。
func (s *BirthdayService) sendWithRetry(ctx context.Context, item *user.User, year int, record *birthday.Log) (string, error) {
	if record.Attempts >= maxBirthdayAttempts {
		return "", ErrBirthdayAttemptsExhausted
	}
	var providerID string
	var sendErr error = ErrBirthdayAttemptsExhausted
	for record.Attempts < maxBirthdayAttempts {
		record.Attempts++
		providerID, sendErr = s.send(ctx, item, year)
		if sendErr == nil {
			return providerID, nil
		}
	}
	return "", sendErr
}

// birthdayOf 优先使用注册时填写的公历出生日期，回退到旧数据的 Birthday 时间戳。
func birthdayOf(item *user.User) time.Time {
	if item.BirthDate != "" {
		if parsed, err := parseBirthDate(item.BirthDate); err == nil {
			return parsed
		}
	}
	return item.Birthday
}

func birthdayMatches(birth, date time.Time) bool {
	month, day := birth.Month(), birth.Day()
	if month == time.February && day == 29 && !isLeap(date.Year()) {
		day = 28
	}
	return date.Month() == month && date.Day() == day
}
func isLeap(year int) bool { return year%400 == 0 || (year%4 == 0 && year%100 != 0) }

func (s *BirthdayService) send(ctx context.Context, item *user.User, year int) (string, error) {
	result, err := s.Sender.Send(ctx, sms.Message{
		AppID:          consts.AppId,
		TemplateCode:   s.Config.BirthdaySms.TemplateCode,
		Phone:          item.Phone,
		Params:         map[string]string{"name": item.Name},
		IdempotencyKey: fmt.Sprintf("birthday:%s:%d", item.ID.Hex(), year),
	})
	if err != nil {
		return "", err
	}
	return result.MessageID, nil
}
