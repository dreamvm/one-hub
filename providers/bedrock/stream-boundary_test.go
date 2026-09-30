package bedrock_test

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"io"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
	"github.com/stretchr/testify/require"

	"one-api/providers/bedrock"
)

func eventFrame(t *testing.T, kind string, payload []byte, extras eventstream.Headers) []byte {
	t.Helper()
	headers := eventstream.Headers{}
	if kind != "" {
		headers.Set(":message-type", eventstream.StringValue(kind))
	}
	for _, header := range extras {
		headers.Set(header.Name, header.Value)
	}
	var out bytes.Buffer
	require.NoError(t, eventstream.NewEncoder().Encode(&out, eventstream.Message{Headers: headers, Payload: payload}))
	return out.Bytes()
}

func binaryFrame(headers []byte) []byte {
	frame := make([]byte, 12+len(headers)+4)
	binary.BigEndian.PutUint32(frame[:4], uint32(len(frame)))
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(headers)))
	binary.BigEndian.PutUint32(frame[8:12], crc32.ChecksumIEEE(frame[:8]))
	copy(frame[12:], headers)
	binary.BigEndian.PutUint32(frame[len(frame)-4:], crc32.ChecksumIEEE(frame[:len(frame)-4]))
	return frame
}

func encodedPayload(t *testing.T, value string) []byte {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"bytes": base64.StdEncoding.EncodeToString([]byte(value))})
	require.NoError(t, err)
	return payload
}

type boundaryBody struct {
	*bytes.Reader
	closed bool
}

func (body *boundaryBody) Close() error {
	body.closed = true
	return nil
}

func readFrames(t *testing.T, frames []byte, wantData []string, wantError string) {
	t.Helper()
	body := &boundaryBody{Reader: bytes.NewReader(frames)}
	resp := &http.Response{Body: body, Header: http.Header{"Content-Type": {"application/vnd.amazon.eventstream"}}}
	stream, requestErr := bedrock.RequestStream(resp, func(line *[]byte, data chan string, _ chan error) {
		data <- string(*line)
	})
	require.Nil(t, requestErr)
	t.Cleanup(stream.Close)
	data, errs := stream.Recv()
	var received []string
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		select {
		case value := <-data:
			received = append(received, value)
		case err := <-errs:
			if wantError == "EOF" {
				require.ErrorIs(t, err, io.EOF)
			} else {
				require.Error(t, err)
				require.NotErrorIs(t, err, io.EOF)
				require.ErrorContains(t, err, wantError)
			}
			require.Equal(t, wantData, received)
			stream.Close()
			require.True(t, body.closed)
			return
		case <-timer.C:
			t.Fatal("stream did not return a terminal error")
		}
	}
}

func TestBedrockStreamBoundary(t *testing.T) {
	for _, name := range []string{
		"normal sequence and header types", "unknown header types", "missing exception type",
		"exception", "error", "error defaults", "missing message type", "unknown message type",
		"invalid JSON", "invalid base64", "invalid checksum", "empty EOF",
	} {
		t.Run(name, func(t *testing.T) {
			// Recv starts its own goroutine. Isolate old worker panics so the
			// complete failure/control matrix can still finish and be inspected.
			command := exec.Command(os.Args[0], "-test.run=^TestBedrockStreamBoundaryHelper$", "-test.timeout=10s")
			command.Env = append(os.Environ(), "ONEHUB_EVENTSTREAM_FIXTURE="+name)
			output, err := command.CombinedOutput()
			require.NoError(t, err, "%s", output)
		})
	}
}

func TestBedrockStreamBoundaryHelper(t *testing.T) {
	name := os.Getenv("ONEHUB_EVENTSTREAM_FIXTURE")
	if name == "" {
		t.Skip("subprocess helper")
	}
	extras := eventstream.Headers{}
	switch name {
	case "normal sequence and header types":
		values := []eventstream.Value{
			eventstream.BoolValue(true), eventstream.BoolValue(false), eventstream.Int8Value(1),
			eventstream.Int16Value(2), eventstream.Int32Value(3), eventstream.Int64Value(4),
			eventstream.BytesValue([]byte("fixture")), eventstream.StringValue("fixture"),
			eventstream.TimestampValue(time.Unix(1700000000, 0)), eventstream.UUIDValue([16]byte{1}),
		}
		var frames []byte
		var want []string
		for index, value := range values {
			extras.Set("x-fixture", value)
			text := "正常-stream-" + string(rune('a'+index))
			frames = append(frames, eventFrame(t, "event", encodedPayload(t, text), extras)...)
			want = append(want, text)
		}
		readFrames(t, frames, want, "EOF")
	case "unknown header types":
		valid := eventFrame(t, "event", nil, nil)
		headers := valid[12 : len(valid)-4]
		for valueType := 10; valueType <= 255; valueType++ {
			invalid := []byte{1, 'x', byte(valueType)}
			readFrames(t, binaryFrame(invalid), nil, "unknown value type")
			combined := append(append([]byte(nil), headers...), invalid...)
			readFrames(t, binaryFrame(combined), nil, "unknown value type")
		}
	case "missing exception type":
		readFrames(t, eventFrame(t, "exception", nil, nil), nil, ":exception-type")
	case "exception":
		extras.Set(":exception-type", eventstream.StringValue("fixtureException"))
		readFrames(t, eventFrame(t, "exception", nil, extras), nil, "fixtureException")
	case "error":
		extras.Set(":error-code", eventstream.StringValue("FixtureError"))
		extras.Set(":error-message", eventstream.StringValue("fixture rejected"))
		readFrames(t, eventFrame(t, "error", nil, extras), nil, "fixture rejected")
	case "error defaults":
		readFrames(t, eventFrame(t, "error", nil, nil), nil, "UnknownError")
	case "missing message type":
		readFrames(t, eventFrame(t, "", nil, nil), nil, ":message-type")
	case "unknown message type":
		readFrames(t, eventFrame(t, "unknown", nil, nil), nil, "unknown error")
	case "invalid JSON":
		readFrames(t, eventFrame(t, "event", []byte("{"), nil), nil, "unexpected end")
	case "invalid base64":
		readFrames(t, eventFrame(t, "event", []byte(`{"bytes":"!"}`), nil), nil, "base64")
	case "invalid checksum":
		frame := eventFrame(t, "event", encodedPayload(t, "fixture"), nil)
		frame[len(frame)-1] ^= 1
		readFrames(t, frame, nil, "checksum")
	case "empty EOF":
		readFrames(t, nil, nil, "EOF")
	default:
		t.Fatalf("unknown fixture %q", name)
	}
}
