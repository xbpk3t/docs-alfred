package pkg

import (
	"testing"
	"time"

	"github.com/golang-module/carbon/v2"
)

func TestFilterFeedsWithTimeRange(t *testing.T) {
	type args struct {
		created  time.Time
		schedule string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"Daily: 当前: true", args{created: time.Now(), schedule: "daily"}, true},
		{"Daily: 昨天feed: true", args{created: carbon.Yesterday().StdTime(), schedule: "daily"}, true},
		{"Daily: 昨天零点feed: true", args{created: carbon.Yesterday().StartOfDay().StdTime(), schedule: "daily"}, true},
		{"Daily: 前天feed: false", args{created: carbon.Now().SubDays(2).StdTime(), schedule: "daily"}, false},
		{"Weekly: 前天feed: true", args{created: carbon.Now().SubDays(2).StdTime(), schedule: "weekly"}, true},
		{"Weekly: 本周feed: true", args{created: carbon.Now().SubDays(7).StdTime(), schedule: "weekly"}, true},
		{"Weekly: 上周之前的feed: false", args{created: carbon.Now().SubDays(8).StdTime(), schedule: "weekly"}, false},
		{"Wekly: false", args{created: carbon.Now().SubDays(8).StdTime(), schedule: "wekly"}, false}, // 拼写错误返回false
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FilterFeedsWithTimeRange(tt.args.created, tt.args.schedule); got != tt.want {
				t.Errorf("FilterFeedsWithTimeRange() = %v, want %v", got, tt.want)
			}
		})
	}
}
