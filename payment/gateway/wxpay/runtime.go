package wxpay

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

type wechatRuntime struct {
	fingerprint [32]byte
	manager     *downloader.CertificateDownloaderMgr
	client      *core.Client
}

func (w *WeChatPay) runtime(ctx context.Context, config *WeChatConfig) (*wechatRuntime, error) {
	if config.MchID == "" || config.MchCertificateSerialNumber == "" || len(config.MchAPIv3Key) != 32 {
		return nil, errors.New("incomplete merchant signing configuration")
	}
	data, _ := json.Marshal([]string{config.MchID, config.MchCertificateSerialNumber, config.MchPrivateKey, config.MchAPIv3Key})
	fingerprint := sha256.Sum256(data)
	w.mu.Lock()
	defer w.mu.Unlock()
	if existing := w.runtimes[config.MchID]; existing != nil && existing.fingerprint == fingerprint {
		return existing, nil
	}
	key, err := utils.LoadPrivateKey(config.MchPrivateKey)
	if err != nil {
		return nil, errors.New("invalid merchant private key")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	manager := downloader.NewCertificateDownloaderMgr(context.Background())
	if err := manager.RegisterDownloaderWithPrivateKey(ctx, key, config.MchCertificateSerialNumber, config.MchID, config.MchAPIv3Key); err != nil {
		manager.Stop()
		return nil, errors.New("merchant certificate initialization failed")
	}
	client, err := core.NewClient(ctx, option.WithWechatPayAutoAuthCipherUsingDownloaderMgr(config.MchID, config.MchCertificateSerialNumber, key, manager))
	if err != nil {
		manager.Stop()
		return nil, err
	}
	runtime := &wechatRuntime{fingerprint: fingerprint, manager: manager, client: client}
	if w.runtimes == nil {
		w.runtimes = make(map[string]*wechatRuntime)
	}
	previous := w.runtimes[config.MchID]
	w.runtimes[config.MchID] = runtime
	// Stop obsolete refresh work. Existing requests keep their certificate map.
	if previous != nil {
		previous.manager.Stop()
	}
	return runtime, nil
}

// Close releases certificate refresh workers owned by this gateway instance.
func (w *WeChatPay) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, runtime := range w.runtimes {
		runtime.manager.Stop()
	}
	w.runtimes = nil
}
