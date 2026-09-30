package stmp_test

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/http/httptest"
	stdmail "net/mail"
	"net/textproto"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"one-api/common"
	"one-api/common/stmp"
)

type envelopeTranscript struct {
	commands []string
	body     string
	err      error
}

// The peer only records a synthetic message on loopback; it never routes mail.
func envelopePeer(t *testing.T, failure string) (int, string, <-chan envelopeTranscript) {
	t.Helper()
	certificateSource := httptest.NewTLSServer(nil)
	serverTLS := certificateSource.TLS.Clone()
	certificate := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateSource.Certificate().Raw}))
	certificateSource.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	done := make(chan envelopeTranscript, 1)
	go func() {
		var result envelopeTranscript
		defer func() { done <- result }()
		conn, err := listener.Accept()
		if err != nil {
			result.err = err
			return
		}
		defer conn.Close()
		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			result.err = err
			return
		}
		reply := func(line string) bool {
			_, result.err = fmt.Fprint(conn, line+"\r\n")
			return result.err == nil
		}
		if !reply("220 localhost fixture") {
			return
		}
		reader := textproto.NewReader(bufio.NewReader(conn))
		for {
			line, err := reader.ReadLine()
			if err != nil {
				result.err = err
				return
			}
			result.commands = append(result.commands, line)
			switch {
			case strings.HasPrefix(line, "EHLO "):
				if !reply("250-localhost\r\n250-STARTTLS\r\n250 AUTH PLAIN") {
					return
				}
			case line == "STARTTLS":
				if !reply("220 start TLS") {
					return
				}
				secure := tls.Server(conn, serverTLS)
				if err := secure.Handshake(); err != nil {
					result.err = err
					return
				}
				conn = secure
				reader = textproto.NewReader(bufio.NewReader(conn))
			case strings.HasPrefix(line, "AUTH PLAIN "):
				response := "235 authenticated"
				if failure == "auth" {
					response = "535 authentication refused"
				}
				if !reply(response) {
					return
				}
			case line == "*" && failure == "auth":
				if !reply("501 authentication aborted") {
					return
				}
			case strings.HasPrefix(line, "MAIL FROM:"):
				if !reply("250 sender accepted") {
					return
				}
			case strings.HasPrefix(line, "RCPT TO:"):
				response := "250 recipient accepted"
				if failure == "recipient" {
					response = "550 recipient refused"
				}
				if !reply(response) {
					return
				}
			case line == "DATA":
				if !reply("354 send fixture") {
					return
				}
				body, err := reader.ReadDotBytes()
				if err != nil {
					result.err = err
					return
				}
				result.body = string(body)
				response := "250 stored locally"
				if failure == "data" {
					response = "554 fixture refused"
				}
				if !reply(response) {
					return
				}
			case line == "RSET", line == "NOOP":
				if !reply("250 OK") {
					return
				}
			case line == "QUIT":
				reply("221 closing")
				return
			default:
				result.err = fmt.Errorf("unexpected SMTP command %q", line)
				return
			}
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port, certificate, done
}

// Trust is restricted to a child process so other tests retain system roots.
// This exercises StmpConfig.Send with its unchanged mandatory TLS policy.
func TestSMTPEnvelopeSendHelper(t *testing.T) {
	certificate := os.Getenv("ONEHUB_SMTP_FIXTURE_CERT")
	if certificate == "" {
		t.Skip("subprocess helper")
	}
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM([]byte(certificate)))
	x509.SetFallbackRoots(roots)
	port, err := strconv.Atoi(os.Getenv("ONEHUB_SMTP_FIXTURE_PORT"))
	require.NoError(t, err)
	client := stmp.NewStmp("127.0.0.1", port, "fixture", "fixture", os.Getenv("ONEHUB_SMTP_FIXTURE_FROM"))
	err = client.Send(os.Getenv("ONEHUB_SMTP_FIXTURE_TO"), "fixture subject", "fixture-body")
	if expected := os.Getenv("ONEHUB_SMTP_FIXTURE_ERROR"); expected != "" {
		require.ErrorContains(t, err, expected)
	} else {
		require.NoError(t, err)
	}
}

func sendEnvelopeFixture(t *testing.T, from, to, failure string) envelopeTranscript {
	t.Helper()
	port, certificate, done := envelopePeer(t, failure)
	expectedError := map[string]string{"auth": "535", "recipient": "550", "data": "554"}[failure]
	command := exec.Command(os.Args[0], "-test.run=^TestSMTPEnvelopeSendHelper$", "-test.timeout=10s")
	command.Env = append(os.Environ(), "GODEBUG=x509usefallbackroots=1",
		"ONEHUB_SMTP_FIXTURE_CERT="+certificate,
		"ONEHUB_SMTP_FIXTURE_PORT="+strconv.Itoa(port),
		"ONEHUB_SMTP_FIXTURE_FROM="+from,
		"ONEHUB_SMTP_FIXTURE_TO="+to,
		"ONEHUB_SMTP_FIXTURE_ERROR="+expectedError)
	output, err := command.CombinedOutput()
	select {
	case transcript := <-done:
		require.NoError(t, err, "%s", output)
		require.NoError(t, transcript.err)
		require.Contains(t, transcript.commands, "STARTTLS")
		return transcript
	case <-time.After(6 * time.Second):
		t.Fatalf("SMTP fixture did not finish: %v %s", err, output)
		return envelopeTranscript{}
	}
}

func TestSMTPEnvelopeAddressEncoding(t *testing.T) {
	for _, test := range []struct {
		name, from, to, wantFrom, wantTo string
	}{
		{"ordinary", "sender@example.test", "recipient@example.test", "<sender@example.test>", "<recipient@example.test>"},
		{"plus tag", "sender@example.test", "recipient+tag@example.test", "<sender@example.test>", "<recipient+tag@example.test>"},
		{"quoted recipient boundary", "sender@example.test", `"local> NOTIFY=NEVER"@example.test`, "<sender@example.test>", `<"local> NOTIFY=NEVER"@example.test>`},
		{"quoted sender boundary", `"local> RET=HDRS"@example.test`, "recipient@example.test", `<"local> RET=HDRS"@example.test>`, "<recipient@example.test>"},
		{"legal space", "sender@example.test", `"two words"@example.test`, "<sender@example.test>", `<"two words"@example.test>`},
		{"escaped quote", "sender@example.test", `"local\"name"@example.test`, "<sender@example.test>", `<"local\"name"@example.test>`},
		{"escaped backslash", "sender@example.test", `"local\\name"@example.test`, "<sender@example.test>", `<"local\\name"@example.test>`},
		{"display names", "Sender <sender@example.test>", "Recipient <recipient@example.test>", "<sender@example.test>", "<recipient@example.test>"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Public verification/reset handlers use this same validator.
			if test.name != "display names" {
				require.NoError(t, common.Validate.Var(test.to, "required,email"))
			}
			transcript := sendEnvelopeFixture(t, test.from, test.to, "")
			var from, to []string
			for _, command := range transcript.commands {
				if strings.HasPrefix(command, "MAIL FROM:") {
					from = append(from, command)
				}
				if strings.HasPrefix(command, "RCPT TO:") {
					to = append(to, command)
				}
			}
			require.Equal(t, []string{"MAIL FROM:" + test.wantFrom}, from)
			require.Equal(t, []string{"RCPT TO:" + test.wantTo}, to)
			require.Contains(t, transcript.body, "fixture-body")
			require.Contains(t, transcript.body, "Subject: fixture subject")
			message, err := stdmail.ReadMessage(strings.NewReader(transcript.body))
			require.NoError(t, err)
			for key, original := range map[string]string{"From": test.from, "To": test.to} {
				want, err := stdmail.ParseAddress(original)
				require.NoError(t, err)
				got, err := stdmail.ParseAddress(message.Header.Get(key))
				require.NoError(t, err)
				require.Equal(t, want, got, "%s header must preserve address and display name", key)
			}
		})
	}
}

func TestSMTPEnvelopeDeliveryFailures(t *testing.T) {
	for _, failure := range []string{"auth", "recipient", "data"} {
		t.Run(failure, func(t *testing.T) {
			transcript := sendEnvelopeFixture(t, "sender@example.test", "recipient@example.test", failure)
			if failure != "data" {
				require.Empty(t, transcript.body)
				require.NotContains(t, transcript.commands, "DATA")
			} else {
				require.Contains(t, transcript.body, "fixture-body")
			}
		})
	}
}
