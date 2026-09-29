package model_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	stripeapi "github.com/stripe/stripe-go/v80"
	"gorm.io/gorm"

	"one-api/model"
	stripegateway "one-api/payment/gateway/stripe"
)

func TestQuotaTransactionPaymentStripeRegistration(t *testing.T) {
	for _, stage := range []string{"legacy create control", "new", "existing", "missing async", "wildcard", "missing secret", "empty key", "list failure", "update failure", "new missing secret", "disabled", "version mismatch", "compatible version", "database failure", "duplicate URL"} {
		t.Run(stage, func(t *testing.T) {
			db, gateway, _, _ := paymentTransactionFixture(t, false)
			conf := stripegateway.StripeConfig{SecretKey: "fixture-stripe-account", WebhookSecret: "fixture-existing-signature"}
			if stage == "legacy create control" || stage == "new" || stage == "new missing secret" || stage == "database failure" || stage == "missing secret" {
				conf.WebhookSecret = ""
			}
			if stage == "empty key" {
				conf.SecretKey = ""
			}
			raw, err := json.Marshal(conf)
			require.NoError(t, err)
			gateway.Type = "stripe"
			gateway.Config = string(raw)
			require.NoError(t, gateway.Update(true))
			if stage == "database failure" {
				require.NoError(t, db.Callback().Update().Before("gorm:update").Register("stripe_config_fault", func(tx *gorm.DB) { tx.AddError(errors.New("fixture config failure")) }))
				t.Cleanup(func() { require.NoError(t, db.Callback().Update().Remove("stripe_config_fault")) })
			}
			oldBackend, oldKey := stripeapi.GetBackend(stripeapi.APIBackend), stripeapi.Key
			t.Cleanup(func() { stripeapi.SetBackend(stripeapi.APIBackend, oldBackend); stripeapi.Key = oldKey })
			stripeapi.Key = "fixture-unrelated-account"
			notify := "https://fixture.invalid/stripe-notify"
			var calls, creates, updates int
			transport := paymentRoundTripper(func(r *http.Request) (*http.Response, error) {
				calls++
				require.Equal(t, "Bearer "+conf.SecretKey, r.Header.Get("Authorization"))
				require.Equal(t, "api.stripe.com", r.URL.Host)
				var body any
				status := 200
				switch {
				case r.Method == "GET" && r.URL.Path == "/v1/webhook_endpoints":
					if stage == "list failure" {
						status = 400
						body = map[string]any{"error": map[string]string{"message": "fixture list failure", "type": "invalid_request_error"}}
						break
					}
					events := []string{"checkout.session.completed", "checkout.session.async_payment_succeeded", "customer.created"}
					if stage == "missing async" || stage == "update failure" {
						events = []string{"checkout.session.completed", "customer.created"}
					}
					if stage == "wildcard" {
						events = []string{"*"}
					}
					webhook := map[string]any{"id": "we_fixture", "url": notify, "enabled_events": events, "status": "enabled", "api_version": stripeapi.APIVersion}
					if stage == "disabled" {
						webhook["status"] = "disabled"
					}
					if stage == "version mismatch" {
						webhook["api_version"] = "2020-08-27"
					}
					if stage == "compatible version" {
						webhook["api_version"] = "2025-02-24.acacia"
					}
					data := []any{webhook}
					if stage == "legacy create control" || stage == "new" || stage == "new missing secret" || stage == "database failure" {
						data = []any{}
					}
					if stage == "duplicate URL" {
						data = append(data, webhook)
					}
					body = map[string]any{"object": "list", "data": data, "has_more": false}
				case r.Method == "POST" && r.URL.Path == "/v1/webhook_endpoints":
					creates++
					require.NoError(t, r.ParseForm())
					require.Contains(t, r.PostForm["enabled_events[0]"], "checkout.session.completed")
					if stage != "legacy create control" {
						require.Contains(t, r.PostForm["enabled_events[1]"], "checkout.session.async_payment_succeeded")
					}
					require.Equal(t, stripeapi.APIVersion, r.FormValue("api_version"))
					secret := "fixture-new-signature"
					if stage == "new missing secret" {
						secret = ""
					}
					body = map[string]string{"id": "we_new", "secret": secret}
				case r.Method == "POST" && r.URL.Path == "/v1/webhook_endpoints/we_fixture":
					updates++
					require.NoError(t, r.ParseForm())
					encoded := r.PostForm.Encode()
					require.Contains(t, encoded, "customer.created")
					require.Contains(t, encoded, "checkout.session.async_payment_succeeded")
					if stage == "update failure" {
						status = 400
						body = map[string]any{"error": map[string]string{"message": "fixture update failure", "type": "invalid_request_error"}}
					} else {
						body = map[string]string{"id": "we_fixture"}
					}
				default:
					return nil, fmt.Errorf("unexpected fixture request")
				}
				data, err := json.Marshal(body)
				if err != nil {
					return nil, err
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(string(data))), Request: r}, nil
			})
			stripeapi.SetBackend(stripeapi.APIBackend, stripeapi.NewBackends(&http.Client{Transport: transport}).API)
			err = (&stripegateway.Stripe{}).CreatedPay(notify, &gateway)
			good := stage == "legacy create control" || stage == "new" || stage == "existing" || stage == "missing async" || stage == "wildcard" || stage == "compatible version"
			var saved model.Payment
			require.NoError(t, db.First(&saved, gateway.ID).Error)
			if good {
				require.NoError(t, err)
				var got stripegateway.StripeConfig
				require.NoError(t, json.Unmarshal([]byte(saved.Config), &got))
				want := conf.WebhookSecret
				if stage == "new" || stage == "legacy create control" {
					want = "fixture-new-signature"
				}
				require.Equal(t, want, got.WebhookSecret)
			} else {
				require.Error(t, err)
				require.Equal(t, string(raw), saved.Config)
				require.Equal(t, string(raw), gateway.Config)
			}
			if stage == "empty key" {
				require.Zero(t, calls)
			}
			if stage == "legacy create control" || stage == "new" || stage == "new missing secret" || stage == "database failure" {
				require.Equal(t, 1, creates)
			} else {
				require.Zero(t, creates)
			}
			if stage == "missing async" || stage == "update failure" {
				require.Equal(t, 1, updates)
			} else {
				require.Zero(t, updates)
			}
			if stage != "legacy create control" {
				require.Equal(t, "fixture-unrelated-account", stripeapi.Key)
			}
		})
	}
}
