package pwgen

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/dop251/goja"
)

//nolint:dupword // JavaScript code contains intentional repetition
const js = `
/**
 * 生成密码
 * @param {sha512加密后字符串} hash
 * @param {输出密码长度} length
 * @param {是否使用标点} rule_of_punctuation
 * @param {是否区分大小写} rule_of_letter
 */
function seekPassword(hash, length, rule_of_punctuation, rule_of_letter) {
	// 生成字符表
	var lower = "abcdefghijklmnopqrstuvwxyz".split("");
	var upper = "ABCDEFGHIJKLMNOPQRSTUVWXYZ".split("");
	var number = "0123456789".split("");
	var punctuation = "~*-+()!@#$^&".split("");
	var alphabet = lower.concat(number);
	if (parseInt(rule_of_punctuation) === 1) {
		alphabet = alphabet.concat(punctuation);
	}
	if (parseInt(rule_of_letter) === 1) {
		alphabet = alphabet.concat(upper);
	}

	// 生成密码
	// 从0开始截取长度为length的字符串，直到满足密码复杂度为止
	for (var i = 0; i <= hash.length - length; ++i) {
		var sub_hash = hash.slice(i, i + parseInt(length)).split("");
		var count = 0;
		var map_index = sub_hash.map(function(c) {
			count = (count + c.charCodeAt()) % alphabet.length;
			return count;
		});
		var sk_pwd = map_index.map(function(k) {
			return alphabet[k];
		});

		// 验证密码
		var matched = [false, false, false, false];
		sk_pwd.forEach(function(e) {
			matched[0] = matched[0] || lower.includes(e);
			matched[1] = matched[1] || upper.includes(e);
			matched[2] = matched[2] || number.includes(e);
			matched[3] = matched[3] || punctuation.includes(e);
		});
		if (parseInt(rule_of_letter) == -1) {
			matched[1] = true;
		}
		if (parseInt(rule_of_punctuation) == -1) {
			matched[3] = true;
		}
		if (!matched.includes(false)) {
			return sk_pwd.join("");
		}
	}
	return "";
}
`

// Generator handles password generation.
type Generator struct {
	config *Config
}

// NewGenerator creates a new password generator.
func NewGenerator(config *Config) *Generator {
	return &Generator{config: config}
}

// Generate generates a password for the given website.
func (g *Generator) Generate(website string) (string, error) {
	hash := g.sha512(g.config.SecretKey, website)
	pwd := g.generatePwd(hash, g.config.Length, g.config.IsPunctuation, g.config.IsUppercase)

	return pwd, nil
}

// sha512加密 = hex_password
// sk 私钥，记忆密码
// website 网站，区分密码.
//
//nolint:revive // Complexity is inherent to the password generation algorithm
func (g *Generator) sha512(sk, website string) string {
	hexOne := computeHmacSha512(sk, website)
	hexTwo := computeHmacSha512("hello", hexOne)
	hexThree := computeHmacSha512("world", hexOne)

	// 字符串转数组
	source := strings.Split(hexTwo, "")
	if source == nil {
		log.Printf("Error converting hex string to array")

		return ""
	}
	rule := strings.Split(hexThree, "")
	if rule == nil {
		log.Printf("Error converting hex string to array")

		return ""
	}

	for i := 0; i < len(source); i++ {
		if i >= len(rule) {
			log.Printf("Error converting hex string to array")

			continue
		}

		// 这里实际上有bug，但是已经没办法fix了
		zx, err := strconv.ParseFloat(source[i], 64)
		if err != nil {
			continue
		}

		if math.IsNaN(zx) {
			str := "whenthecatisawaythemicewillplay666"
			if !strings.Contains(str, rule[i]) {
				source[i] = strings.ToUpper(source[i])
			}
		}
	}

	return strings.Join(source, "")
}

// 生成密码.
func (g *Generator) generatePwd(hash string, length int, isPunc, isUseUpper bool) string {
	vm := goja.New()
	_, err := vm.RunString(js)
	if err != nil {
		log.Println(err)

		return ""
	}
	var seekPassword func(hash string, length, isPunc, isUpper int) string
	err = vm.ExportTo(vm.Get("seekPassword"), &seekPassword)
	if err != nil {
		log.Println(err)
		panic(err)
	}

	puncFlag := -1
	if isPunc {
		puncFlag = 1
	}

	upperFlag := -1
	if isUseUpper {
		upperFlag = 1
	}

	password := seekPassword(hash, length, puncFlag, upperFlag)

	return password
}

// computeHmacSha512 HmacSha512签名
// message 签名内容
// secret 签名秘钥.
func computeHmacSha512(message, secret string) string {
	key := []byte(secret)
	h := hmac.New(sha512.New, key)
	h.Write([]byte(message))

	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
