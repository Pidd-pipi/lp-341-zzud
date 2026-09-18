package dto

import (
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestFollowUpRequestValidation(t *testing.T) {
	v := validator.New()
	v.SetTagName("binding") // gin 使用 binding 标签
	cases := []struct {
		name    string
		req     FollowUpRequest
		wantErr bool
	}{
		{"done without advice rejected", FollowUpRequest{Status: "done"}, true},
		{"done with advice ok", FollowUpRequest{Status: "done", Advice: "心内科复查"}, false},
		{"pending without advice ok", FollowUpRequest{Status: "pending"}, false},
		{"advice over 500 rejected", FollowUpRequest{Status: "pending", Advice: strings.Repeat("a", 501)}, true},
	}
	for _, c := range cases {
		if err := v.Struct(c.req); (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, wantErr = %v", c.name, err, c.wantErr)
		}
	}
}
