package vertexai_test

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type iamTestServer struct {
	credentialspb.UnimplementedIAMCredentialsServer
	generate func(context.Context, *credentialspb.GenerateAccessTokenRequest) (*credentialspb.GenerateAccessTokenResponse, error)
}

func (s *iamTestServer) GenerateAccessToken(ctx context.Context, req *credentialspb.GenerateAccessTokenRequest) (*credentialspb.GenerateAccessTokenResponse, error) {
	return s.generate(ctx, req)
}

func newIAMTestClient(t *testing.T, server *iamTestServer) *credentials.IamCredentialsClient {
	t.Helper()
	listener := bufconn.Listen(256 * 1024)
	grpcServer := grpc.NewServer()
	credentialspb.RegisterIAMCredentialsServer(grpcServer, server)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		if err := grpcServer.Serve(listener); err != nil {
			t.Errorf("serve IAM fixture: %v", err)
		}
	}()
	t.Cleanup(func() {
		grpcServer.Stop()
		listener.Close()
		<-stopped
	})

	// Exercise SDK construction and its default call options. The custom dialer
	// always uses memory; authentication/TLS and provider caching are not under test.
	client, err := credentials.NewIamCredentialsClient(context.Background(),
		option.WithEndpoint("iam.fixture.invalid:443"),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		option.WithGRPCDialOption(grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		})),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	return client
}

// SDK compatibility for the dependency boundary used by VertexAI GetToken.
// This does not claim that GetToken forwards HTTP request cancellation.
func TestVertexIAMCompatibility(t *testing.T) {
	wantRequest := &credentialspb.GenerateAccessTokenRequest{
		Name:  "projects/-/serviceAccounts/fixture@example.invalid",
		Scope: []string{"https://www.googleapis.com/auth/cloud-platform"},
	}
	expiry := timestamppb.New(time.Unix(1900000000, 123456000))
	for _, tc := range []struct {
		name  string
		token string
		retry bool
	}{
		{name: "normal", token: "fixture-token"},
		{name: "multi-frame-response", token: strings.Repeat("abcdef", 32768)},
		{name: "retry-unavailable", token: "fixture-retried-token", retry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			client := newIAMTestClient(t, &iamTestServer{generate: func(_ context.Context, req *credentialspb.GenerateAccessTokenRequest) (*credentialspb.GenerateAccessTokenResponse, error) {
				if !proto.Equal(req, wantRequest) {
					t.Errorf("unexpected IAM request: %v", req)
					return nil, status.Error(codes.InvalidArgument, "fixture request mismatch")
				}
				if calls.Add(1) == 1 && tc.retry {
					return nil, status.Error(codes.Unavailable, "fixture transient error")
				}
				return &credentialspb.GenerateAccessTokenResponse{AccessToken: tc.token, ExpireTime: expiry}, nil
			}})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			response, err := client.GenerateAccessToken(ctx, wantRequest)
			require.NoError(t, err)
			require.Equal(t, tc.token, response.AccessToken)
			require.True(t, proto.Equal(expiry, response.ExpireTime))
			wantCalls := int32(1)
			if tc.retry {
				wantCalls = 2
			}
			require.Equal(t, wantCalls, calls.Load())
		})
	}

	for _, code := range []codes.Code{codes.PermissionDenied, codes.Unauthenticated} {
		t.Run(code.String(), func(t *testing.T) {
			var calls atomic.Int32
			client := newIAMTestClient(t, &iamTestServer{generate: func(context.Context, *credentialspb.GenerateAccessTokenRequest) (*credentialspb.GenerateAccessTokenResponse, error) {
				calls.Add(1)
				return nil, status.Error(code, "fixture rejected request")
			}})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			response, err := client.GenerateAccessToken(ctx, wantRequest)
			require.Nil(t, response)
			require.Equal(t, code, status.Code(err))
			require.Equal(t, int32(1), calls.Load())
		})
	}

	for _, code := range []codes.Code{codes.Canceled, codes.DeadlineExceeded} {
		t.Run(code.String(), func(t *testing.T) {
			entered := make(chan struct{}, 1)
			exited := make(chan struct{}, 1)
			client := newIAMTestClient(t, &iamTestServer{generate: func(ctx context.Context, _ *credentialspb.GenerateAccessTokenRequest) (*credentialspb.GenerateAccessTokenResponse, error) {
				entered <- struct{}{}
				<-ctx.Done()
				exited <- struct{}{}
				return nil, status.FromContextError(ctx.Err()).Err()
			}})
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			result := make(chan error, 1)
			go func() {
				_, err := client.GenerateAccessToken(ctx, wantRequest)
				result <- err
			}()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("RPC never entered the IAM fixture")
			}
			if code == codes.Canceled {
				cancel()
			}
			select {
			case err := <-result:
				// GAX may return the caller's context error while waiting to retry.
				contextError := context.Canceled
				if code == codes.DeadlineExceeded {
					contextError = context.DeadlineExceeded
				}
				require.True(t, status.Code(err) == code || errors.Is(err, contextError), "unexpected IAM termination: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("IAM call did not terminate")
			}
			select {
			case <-exited:
			case <-time.After(5 * time.Second):
				t.Fatal("IAM handler did not observe cancellation")
			}
		})
	}
}
