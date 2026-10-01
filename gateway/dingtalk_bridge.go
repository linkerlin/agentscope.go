// gateway/dingtalk_bridge.go realises the gateway half of the DingTalk
// closed loop (18.3):
//
//   - enterprise-app credentials come from the 18.4 binding store, never
//     from plaintext process configuration: ResolveDingtalkCredential reads
//     the referenced service.Credential, refuses anything not AUTHORIZED,
//     and decrypts through the configured cipher;
//   - card callbacks are signature-verified (the platform's HMAC pair —
//     that is the "认证" for a machine-to-platform callback) and the HITL
//     decision is injected through the 23.2 idempotent resume state
//     machine — duplicate callbacks and cross-replica consumers are safe
//     by construction.
package gateway

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/linkerlin/agentscope.go/channel/dingtalk"
	"github.com/linkerlin/agentscope.go/service"
)

// dingtalkCredentialData is the secret-reference payload inside an 18.4
// binding credential (delivered at authorize time, encrypted at rest): the
// enterprise-app key/secret pair plus the optional outgoing-robot callback
// secret. References only — the plaintext exists transiently in memory.
type dingtalkCredentialData struct {
	AppKey      string `json:"app_key"`
	AppSecret   string `json:"app_secret"`
	RobotSecret string `json:"robot_secret,omitempty"`
}

// ResolveDingtalkCredential materialises a DingTalk channel config from an
// 18.4 credential. Only AUTHORIZED (or legacy blank-status) credentials
// resolve; PENDING bindings are refused — an unfinished binding must never
// yield working platform credentials. The returned secret lives only for the
// caller's constructor call.
func ResolveDingtalkCredential(ctx context.Context, storage service.Storage, cipher *service.Cipher, credentialID string) (appKey, appSecret, robotSecret string, err error) {
	cred, err := storage.GetCredential(ctx, credentialID)
	if err != nil {
		return "", "", "", fmt.Errorf("dingtalk: credential %s: %w", credentialID, err)
	}
	if st := cred.NormalizedStatus(); st != service.CredentialAuthorized {
		return "", "", "", fmt.Errorf("dingtalk: credential %s is %s, not usable (18.4 binding incomplete)", credentialID, st)
	}
	raw := cred.Encrypted
	if cipher != nil {
		plain, derr := cipher.Decrypt(raw)
		if derr != nil {
			return "", "", "", fmt.Errorf("dingtalk: decrypt credential %s: %w", credentialID, derr)
		}
		raw = plain
	}
	var data dingtalkCredentialData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return "", "", "", fmt.Errorf("dingtalk: credential %s payload: %w", credentialID, err)
	}
	if data.AppKey == "" || data.AppSecret == "" {
		return "", "", "", fmt.Errorf("dingtalk: credential %s missing app_key/app_secret", credentialID)
	}
	return data.AppKey, data.AppSecret, data.RobotSecret, nil
}

// NewDingtalkChannelFromCredential builds the channel from an 18.4 binding.
func NewDingtalkChannelFromCredential(ctx context.Context, id, credentialID string, storage service.Storage, cipher *service.Cipher) (*dingtalk.Channel, error) {
	appKey, appSecret, robotSecret, err := ResolveDingtalkCredential(ctx, storage, cipher, credentialID)
	if err != nil {
		return nil, err
	}
	ch := dingtalk.New(id, appKey, appSecret)
	if robotSecret != "" {
		ch = ch.WithRobotSecret(robotSecret)
	}
	return ch, nil
}
