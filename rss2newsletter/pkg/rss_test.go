package pkg

import (
	"testing"
	"time"

	"github.com/golang-module/carbon/v2"
)

func TestFilterFeedsWithTimeRange(t *testing.T) {
	endDate := carbon.CreateFromDate(2024, 11, 17).StdTime()

	type args struct {
		created  time.Time
		endDate  time.Time
		schedule string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// {"Daily: 当前: true", args{created: time.Now(), endDate: endDate, schedule: "daily"}, true},
		// {"Daily: 昨天feed: true", args{created: carbon.Yesterday().StdTime(), endDate: endDate, schedule: "daily"}, true},
		// {"Daily: 昨天零点feed: true", args{created: carbon.Yesterday().StartOfDay().StdTime(), endDate: endDate, schedule: "daily"}, true},

		{"Daily: 当前: true", args{created: time.Now(), endDate: endDate, schedule: "daily"}, true},
		{"Daily: 昨天feed: true", args{created: carbon.CreateFromDateTime(2024, 11, 16, 10, 10, 10).StdTime(), endDate: endDate, schedule: "daily"}, true},
		{"Daily: 昨天零点feed: true", args{created: carbon.CreateFromDate(2024, 11, 16).StdTime(), endDate: endDate, schedule: "daily"}, true},
		{"Daily: 前天feed: false", args{created: carbon.CreateFromDate(2024, 11, 15).StdTime(), endDate: endDate, schedule: "daily"}, false},
		{"Daily: 前天feed: false", args{created: carbon.CreateFromDateTime(2024, 11, 15, 10, 10, 10).StdTime(), endDate: endDate, schedule: "daily"}, false},
		{"Daily: 前天feed: false", args{created: carbon.CreateFromDateTime(2024, 11, 15, 23, 52, 36).StdTime(), endDate: endDate, schedule: "daily"}, false},

		{"Weekly: 前天feed: true", args{created: carbon.Now().SubDays(2).StdTime(), endDate: endDate, schedule: "weekly"}, true},
		{"Weekly: 本周feed: true", args{created: carbon.Now().SubDays(7).StdTime(), endDate: endDate, schedule: "weekly"}, true},
		// {"Weekly: 上周之前的feed: false", args{created: carbon.Now().SubDays(8).StdTime(), endDate: endDate, schedule: "weekly"}, false},
		{"Wekly: 拼写错误返回false: false", args{created: carbon.Now().SubDays(8).StdTime(), endDate: endDate, schedule: "wekly"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FilterFeedsWithTimeRange(tt.args.created, tt.args.endDate, tt.args.schedule); got != tt.want {
				t.Errorf("FilterFeedsWithTimeRange() = %v, want %v", got, tt.want)
			}
		})
	}
}
