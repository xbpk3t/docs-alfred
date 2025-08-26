package classifier

import (
	"io/ioutil"
	"strings"

	"gopkg.in/yaml.v2"
)

// CategoryConfig 分类配置结构
type CategoryConfig struct {
	支出 map[string]CategoryRule `yaml:"支出"`
	收入 map[string]CategoryRule `yaml:"收入"`
}

// CategoryRule 分类规则
type CategoryRule struct {
	备注   []string `yaml:"备注"`
	交易对方 []string `yaml:"交易对方"`
	交易类型 []string `yaml:"交易类型,omitempty"`
}

// Classifier 分类器
type Classifier struct {
	config CategoryConfig
}

// NewClassifier 创建新的分类器
func NewClassifier(configPath string) (*Classifier, error) {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config CategoryConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &Classifier{
		config: config,
	}, nil
}

// Classify 对账单记录进行分类
func (c *Classifier) Classify(ioType, payType2, payUser, shop string) string {
	// 根据收支类型选择对应的分类规则
	var categories map[string]CategoryRule
	if ioType == "支出" {
		categories = c.config.支出
	} else if ioType == "收入" {
		categories = c.config.收入
	} else {
		return "其它"
	}

	// 遍历分类规则
	for category, rule := range categories {
		// 检查交易对方
		for _, user := range rule.交易对方 {
			if strings.Contains(payUser, user) {
				return category
			}
		}

		// 检查备注
		for _, remark := range rule.备注 {
			if strings.Contains(shop, remark) {
				return category
			}
		}

		// 检查交易类型
		for _, transType := range rule.交易类型 {
			if strings.Contains(payType2, transType) {
				return category
			}
		}
	}

	return "其它"
}
