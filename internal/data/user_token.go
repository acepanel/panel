package data

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/utils/str"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
	"github.com/acepanel/panel/v3/pkg/clientip"
	"github.com/acepanel/panel/v3/pkg/config"
)

type userTokenRepo struct {
	t    *gotext.Locale
	conf *config.Config
	db   *gorm.DB
}

func NewUserTokenRepo(conf *config.Config, db *gorm.DB, t *gotext.Locale) biz.UserTokenRepo {
	return &userTokenRepo{
		t:    t,
		conf: conf,
		db:   db,
	}
}

func (r userTokenRepo) List(userID, page, limit uint) ([]*biz.UserToken, int64, error) {
	userTokens := make([]*biz.UserToken, 0)
	var total int64
	err := r.db.Model(&biz.UserToken{}).Where("user_id = ?", userID).Order("id desc").Count(&total).Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&userTokens).Error
	return userTokens, total, err
}

func (r userTokenRepo) Get(id uint) (*biz.UserToken, error) {
	userToken := new(biz.UserToken)
	if err := r.db.First(userToken, id).Error; err != nil {
		return nil, err
	}

	return userToken, nil
}

func (r userTokenRepo) Create(userID uint, ips []string, expired time.Time) (*biz.UserToken, error) {
	token := str.Random(32)
	userToken := &biz.UserToken{
		UserID:    userID,
		Token:     token,
		IPs:       ips,
		ExpiredAt: expired,
	}
	if err := r.db.Create(userToken).Error; err != nil {
		return nil, err
	}

	userToken.Token = token // 返回的值是加密的，这里覆盖为原始值

	return userToken, nil
}

func (r userTokenRepo) Update(id uint, ips []string, expired time.Time) (*biz.UserToken, error) {
	userToken := new(biz.UserToken)
	if err := r.db.First(userToken, id).Error; err != nil {
		return nil, err
	}

	userToken.IPs = ips
	userToken.ExpiredAt = expired

	if err := r.db.Save(userToken).Error; err != nil {
		return nil, err
	}

	return userToken, nil
}

func (r userTokenRepo) Delete(id uint) error {
	userToken := new(biz.UserToken)
	if err := r.db.First(userToken, id).Error; err != nil {
		return err
	}

	return r.db.Delete(userToken).Error
}

func (r userTokenRepo) ValidateReq(req *http.Request) (uint, error) {
	// Authorization: HMAC-SHA256 Credential=<token_id>, Signature=<signature>
	var algorithm string
	var id uint
	var signature string
	if _, err := fmt.Sscanf(req.Header.Get("Authorization"), "%s Credential=%d, Signature=%s", &algorithm, &id, &signature); err != nil {
		return 0, errors.New(r.t.Get("invalid header: %v", err))
	}
	if algorithm != "HMAC-SHA256" {
		return 0, errors.New(r.t.Get("invalid signature"))
	}

	// 获取用户令牌
	userToken, err := r.Get(id)
	if err != nil {
		return 0, errors.New(r.t.Get("invalid signature")) // 不应返回原始报错，防止猜测令牌ID
	}
	if userToken.ExpiredAt.Before(time.Now()) {
		return 0, errors.New(r.t.Get("token expired"))
	}

	// 步骤一：验证时间戳与来源 IP
	timestamp := cast.ToInt64(req.Header.Get("X-Timestamp"))
	if skew := time.Now().Unix() - timestamp; skew > 300 || skew < -300 {
		return 0, errors.New(r.t.Get("signature expired"))
	}
	if len(userToken.IPs) > 0 {
		ip := clientip.FromRequest(req, r.conf.HTTP.IPHeader, r.conf.HTTP.TrustedProxies)
		if !clientip.Match(ip, userToken.IPs) {
			return 0, errors.New(r.t.Get("invalid request ip: %s", ip))
		}
	}

	// 步骤二：构造规范化请求
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return 0, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s", req.Method, req.URL.Path, req.URL.Query().Encode(), str.SHA256(string(body)))

	// 步骤三：构造待签名字符串
	stringToSign := fmt.Sprintf("%s\n%d\n%s", "HMAC-SHA256", timestamp, str.SHA256(canonicalRequest))

	// 步骤四：计算并验证签名
	validSignature := r.hmacsha256(stringToSign, userToken.Token)
	if subtle.ConstantTimeCompare([]byte(signature), []byte(validSignature)) != 1 {
		return 0, errors.New(r.t.Get("invalid signature"))
	}

	return userToken.UserID, nil
}

func (r userTokenRepo) hmacsha256(data string, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}
