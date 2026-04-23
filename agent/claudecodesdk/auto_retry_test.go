package claudecodesdk

import (
	"errors"
	"testing"
)

func TestIsRetryableError(t *testing.T) {
	s := &sdkSession{}
	s.alive.Store(true)

	tests := []struct {
		name    string
		err     error
		want    bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "API Error 400 with code 1234",
			err:  errors.New(`API Error: 400 {"type":"error","error":{"message":"网络错误，错误id：202604221559261bce6b4e15444051，请稍后重试","code":"1234"},"request_id":"202604221559261bce6b4e15444051"}`),
			want: true,
		},
		{
			name: "API Error 400 generic",
			err:  errors.New(`API Error: 400 {"type":"error","error":{"message":"overloaded"}}`),
			want: true,
		},
		{
			name: "error with code 1234",
			err:  errors.New(`something went wrong {"code":"1234"}`),
			want: true,
		},
		{
			name: "网络错误 without API Error 400",
			err:  errors.New(`网络错误，请稍后重试`),
			want: true,
		},
		{
			name: "non-retryable error",
			err:  errors.New(`API Error: 401 unauthorized`),
			want: false,
		},
		{
			name: "query aborted",
			err:  errors.New(`query aborted`),
			want: false,
		},
		{
			name: "generic error",
			err:  errors.New(`something went wrong`),
			want: false,
		},
		{
			name: "API Error 500",
			err:  errors.New(`API Error: 500 internal server error`),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.IsRetryableError(tt.err)
			if got != tt.want {
				t.Errorf("IsRetryableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
