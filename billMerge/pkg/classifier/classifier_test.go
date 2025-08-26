package classifier

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClassifier(t *testing.T) {
	// 测试分类器创建
	configPath := filepath.Join("..", "..", "..", "billMerge", "config", "category.yaml")

	classifier, err := NewClassifier(configPath)
	// 如果配置文件不存在，则跳过测试
	if err != nil {
		t.Skip("跳过测试，未找到分类配置文件:", err)
	}

	assert.NoError(t, err)
	assert.NotNil(t, classifier)

	// 验证配置加载
	assert.NotEmpty(t, classifier.config.支出)
	assert.NotEmpty(t, classifier.config.收入)
}

func TestClassify(t *testing.T) {
	// 测试分类功能
	configPath := filepath.Join("..", "..", "..", "billMerge", "config", "category.yaml")

	classifier, err := NewClassifier(configPath)
	// 如果配置文件不存在，则跳过测试
	if err != nil {
		t.Skip("跳过测试，未找到分类配置文件:", err)
	}

	assert.NoError(t, err)

	// 测试餐饮分类 - 通过交易对方
	category := classifier.Classify("支出", "", "肯德基", "")
	assert.Equal(t, "餐饮", category)

	// 测试餐饮分类 - 通过备注
	category = classifier.Classify("支出", "", "", "肯德基汉堡")
	assert.Equal(t, "餐饮", category)

	// 测试购物分类 - 通过备注
	category = classifier.Classify("支出", "", "", "京东购物")
	assert.Equal(t, "购物", category)

	// 测试红包分类 - 通过交易类型
	category = classifier.Classify("收入", "红包", "", "")
	assert.Equal(t, "红包", category)

	// 测试未知分类
	category = classifier.Classify("支出", "", "未知商家", "")
	assert.Equal(t, "其它", category)

	// 测试空收支类型
	category = classifier.Classify("", "", "肯德基", "")
	assert.Equal(t, "其它", category)
}
