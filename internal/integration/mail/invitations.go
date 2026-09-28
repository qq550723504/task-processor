package mail

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	gomail "github.com/wneessen/go-mail"
	"net"
	stdmail "net/mail"
	"net/url"
	"strings"
	flow "task-processor/internal/organization/membership/inviteflow"
	"time"
)

type Config struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	From           string `json:"from"`
	PublicOrigin   string `json:"publicOrigin"`
	LocalPlaintext bool   `json:"localPlaintext"`
}
type Sender struct{ config Config }

func New(cfg Config) (*Sender, error) {
	bad := errors.New("invitation mail configuration invalid")
	u, err := url.Parse(cfg.PublicOrigin)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
		return nil, bad
	}
	address, err := stdmail.ParseAddress(cfg.From)
	if err != nil || address.Address != cfg.From || len(cfg.From) > 200 || strings.ContainsAny(cfg.From, "\r\n") {
		return nil, bad
	}
	if cfg.Host == "" || len(cfg.Host) > 253 || strings.ContainsAny(cfg.Host, " /\\\r\n\t:") || cfg.Port < 1 || cfg.Port > 65535 || len(cfg.Username) > 256 || len(cfg.Password) > 4096 || (cfg.Username == "") != (cfg.Password == "") {
		return nil, bad
	}
	if cfg.LocalPlaintext && (cfg.Host != "localhost" && cfg.Host != "127.0.0.1") {
		return nil, bad
	}
	if cfg.LocalPlaintext && cfg.Username != "" {
		return nil, bad
	}
	return &Sender{cfg}, nil
}
func (s *Sender) Send(parent context.Context, inv flow.Invitation) error {
	if s == nil {
		return errors.New("mail unavailable")
	}
	id, err := uuid.Parse(inv.ID)
	if err != nil || id.String() != inv.ID || inv.State != "" && inv.State != flow.Pending {
		return errors.New("invalid mail intent")
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	policy := gomail.TLSMandatory
	if s.config.LocalPlaintext {
		policy = gomail.NoTLS
	}
	// go-mail's dial context is cancelled on completion of dialing. Bind the
	// connection to the outer request instead, including DATA and final ACK.
	dial := func(dialCtx context.Context, network, address string) (net.Conn, error) {
		conn, err := (&net.Dialer{}).DialContext(dialCtx, network, address)
		if err != nil {
			return nil, err
		}
		stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
		return &cancelConn{Conn: conn, stop: stop}, nil
	}
	options := []gomail.Option{gomail.WithPort(s.config.Port), gomail.WithTimeout(10 * time.Second), gomail.WithTLSPolicy(policy), gomail.WithDialContextFunc(dial)}
	if s.config.Username != "" {
		options = append(options, gomail.WithSMTPAuth(gomail.SMTPAuthAutoDiscover), gomail.WithUsername(s.config.Username), gomail.WithPassword(s.config.Password))
	}
	client, err := gomail.NewClient(s.config.Host, options...)
	if err != nil {
		return errors.New("mail unavailable")
	}
	msg := gomail.NewMsg()
	if err = msg.From(s.config.From); err != nil {
		return errors.New("mail unavailable")
	}
	if err = msg.To(inv.Contact); err != nil {
		return errors.New("invalid recipient")
	}
	msg.Subject("硕米智能引擎 · 企业成员邀请")
	roleName := map[string]string{"listingkit_viewer": "只读成员", "listingkit_operator": "操作成员", "listingkit_admin": "企业管理员"}[inv.Role]
	msg.SetBodyString(gomail.TypeTextPlain, fmt.Sprintf("你收到企业 %s 的成员邀请。\n邀请角色：%s\n请登录并验证此邮箱后，确认接受或拒绝：\n%s/invitations/%s\n链接有效期至 %s。未接受前不会成为企业成员。\n", inv.OrganizationName, roleName, s.config.PublicOrigin, inv.ID, inv.ExpiresAt.UTC().Format(time.RFC3339)))
	if err = client.DialAndSendWithContext(ctx, msg); err != nil {
		return errors.New("mail delivery unconfirmed")
	}
	return nil
}

type cancelConn struct {
	net.Conn
	stop func() bool
}

func (c *cancelConn) Close() error { c.stop(); return c.Conn.Close() }
