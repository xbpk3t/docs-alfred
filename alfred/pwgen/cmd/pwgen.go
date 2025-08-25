// Package cmd /*
package cmd

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/dop251/goja"
	"github.com/spf13/cobra"
)

type Config struct {
	SecretKey     string
	Length        int
	IsUppercase   bool
	IsNum         bool
	IsPunctuation bool
	IsTouchId     bool
}

func NewConfig() *Config {
	return &Config{
		Length:        wf.Config.GetInt("length"),
		IsUppercase:   wf.Config.GetBool("uppercase"),
		IsNum:         wf.Config.GetBool("numbers"),
		IsPunctuation: wf.Config.GetBool("punctuation"),
		SecretKey:     wf.Config.GetString("sk"),
	}
}

// // pwgenCmd represents the pwgen command
//
//	var pwgenCmd = &cobra.Command{
//		Use:   "pwgen",
//		Short: "A brief description of your command",
//		Run: func(cmd *cobra.Command, args []string) {
//			c := NewConfig()
//
//			// if c.IsTouchId {
//			// 	auth, err := touchid.Auth(touchid.DeviceTypeAny, "Touch Id")
//			// 	if err != nil {
//			// 		return
//			// 	}
//			// 	if !auth {
//			// 		return
//			// 	}
//			// }
//
//			hash := Sha512(c.SecretKey, args[0])
//			pwd := GeneratePwd(hash, c.Length, c.IsPunctuation, c.IsUppercase)
//			// fmt.Println(pwd)
//			wf.NewItem(pwd).Title(pwd).Arg(pwd).Valid(true)
//			wf.SendFeedback()
//		},
//	}
func init() {
	rootCmd.AddCommand(pwgenCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// pwgenCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// pwgenCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

//
// // sha512加密 = hex_password
// // sk 私钥，记忆密码
// // website 网站，区分密码
// func Sha512(sk, website string) string {
// 	hexOne := ComputeHmacSha512(sk, website)
// 	hexTwo := ComputeHmacSha512("hello", hexOne)
// 	hexThree := ComputeHmacSha512("world", hexOne)
//
// 	// 字符串转数组
// 	source := strings.Split(hexTwo, "")
// 	rule := strings.Split(hexThree, "")
//
// 	for i := 0; i < len(source); i++ {
// 		zx, _ := strconv.ParseFloat(source[i], 64)
//
// 		if math.IsNaN(zx) {
// 			str := "whenthecatisawaythemicewillplay666"
// 			if !strings.Contains(str, rule[i]) {
// 				source[i] = strings.ToUpper(source[i])
// 			}
// 		}
// 	}
//
// 	return strings.Join(source, "")
// }
//
// // 生成密码
// func GeneratePwd(hash string, length int, isPunc, isUseUpper bool) string {
// 	vm := goja.New()
// 	_, err := vm.RunString(js)
// 	if err != nil {
// 		log.Println(err)
// 		return ""
// 	}
// 	var seekPassword func(hash string, length, isPunc, isUpper int) string
// 	err = vm.ExportTo(vm.Get("seekPassword"), &seekPassword)
// 	if err != nil {
// 		log.Println(err)
// 		panic(err)
// 	}
//
// 	password := seekPassword(hash, 16, -1, 1)
// 	// log.Println(password)
//
// 	return password
// }
//
// // ComputeHmacSha512 HmacSha512签名
// // message 签名内容
// // secret 签名秘钥
// func ComputeHmacSha512(message string, secret string) string {
// 	key := []byte(secret)
// 	h := hmac.New(sha512.New, key)
// 	h.Write([]byte(message))
// 	return base64.StdEncoding.EncodeToString(h.Sum(nil))
// }
//
// const js = `
// /**
//  * 生成密码
//  * @param {sha512加密后字符串} hash
//  * @param {输出密码长度} length
//  * @param {是否使用标点} rule_of_punctuation
//  * @param {是否区分大小写} rule_of_letter
//  */
// function seekPassword(hash, length, rule_of_punctuation, rule_of_letter) {
// 	// 生成字符表
// 	var lower = "abcdefghijklmnopqrstuvwxyz".split("");
// 	var upper = "ABCDEFGHIJKLMNOPQRSTUVWXYZ".split("");
// 	var number = "0123456789".split("");
// 	var punctuation = "~*-+()!@#$^&".split("");
// 	var alphabet = lower.concat(number);
// 	if (parseInt(rule_of_punctuation) === 1) {
// 		alphabet = alphabet.concat(punctuation);
// 	}
// 	if (parseInt(rule_of_letter) === 1) {
// 		alphabet = alphabet.concat(upper);
// 	}
//
// 	// 生成密码
// 	// 从0开始截取长度为length的字符串，直到满足密码复杂度为止
// 	for (var i = 0; i <= hash.length - length; ++i) {
// 		var sub_hash = hash.slice(i, i + parseInt(length)).split("");
// 		var count = 0;
// 		var map_index = sub_hash.map(function(c) {
// 			count = (count + c.charCodeAt()) % alphabet.length;
// 			return count;
// 		});
// 		var sk_pwd = map_index.map(function(k) {
// 			return alphabet[k];
// 		});
//
// 		// 验证密码
// 		var matched = [false, false, false, false];
// 		sk_pwd.forEach(function(e) {
// 			matched[0] = matched[0] || lower.includes(e);
// 			matched[1] = matched[1] || upper.includes(e);
// 			matched[2] = matched[2] || number.includes(e);
// 			matched[3] = matched[3] || punctuation.includes(e);
// 		});
// 		if (parseInt(rule_of_letter) == -1) {
// 			matched[1] = true;
// 		}
// 		if (parseInt(rule_of_punctuation) == -1) {
// 			matched[3] = true;
// 		}
// 		if (!matched.includes(false)) {
// 			return sk_pwd.join("");
// 		}
// 	}
// 	return "";
// }
// `

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

// pwgenCmd represents the pwgen command
var pwgenCmd = &cobra.Command{
	Use:   "pwgen",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		c := NewConfig()

		website := args[0]
		hash := Sha512(c.SecretKey, website)
		pwd := GeneratePwd(hash, c.Length, c.IsPunctuation, c.IsUppercase)

		wf.NewItem(pwd).Title(pwd).Arg(pwd).Valid(true)
		wf.SendFeedback()
	},
}

// func Gen(website string) string {
// 	// 使用viper从config.toml读取数据
// 	viper.SetConfigName("config")
// 	viper.SetConfigType("toml")
// 	viper.AddConfigPath(".")
// 	err := viper.ReadInConfig()
// 	if err != nil {
// 		log.Printf("Error reading config file, %s", err)
// 	}
//
// 	sk := viper.GetString("sk")
// 	isPunctuation := viper.GetBool("punctuation")
// 	isUpper := viper.GetBool("uppercase")
// 	pwdLength := viper.GetInt("length")
//
// 	if sk != "" && website != "" {
// 		hash := Sha512(sk, website)
// 		return GeneratePwd(hash, pwdLength, isPunctuation, isUpper)
// 	}
//
// 	return ""
// }

// sha512加密 = hex_password
// sk 私钥，记忆密码
// website 网站，区分密码
func Sha512(sk, website string) string {
	hexOne := ComputeHmacSha512(sk, website)
	hexTwo := ComputeHmacSha512("hello", hexOne)
	hexThree := ComputeHmacSha512("world", hexOne)

	// 字符串转数组
	source := strings.Split(hexTwo, "")
	if source == nil {
		// 处理错误，例如返回错误信息或者初始化source为一个空的切片
		log.Printf("Error converting hex string to array")
		return ""
	}
	rule := strings.Split(hexThree, "")
	if rule == nil {
		// 同样处理错误
		log.Printf("Error converting hex string to array")
		return ""
	}

	for i := 0; i < len(source); i++ {
		if i >= len(rule) { // 确保不会访问rule数组的越界索引
			log.Printf("Error converting hex string to array")
			continue
		}
		zx, err := strconv.ParseFloat(source[i], 64)
		if err != nil {
			log.Printf("Error converting hex string to array: +%v", err)
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

// 生成密码
func GeneratePwd(hash string, length int, isPunc, isUseUpper bool) string {
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

	password := seekPassword(hash, 16, -1, 1)
	log.Println(password)
	return password
}

// ComputeHmacSha512 HmacSha512签名
// message 签名内容
// secret 签名秘钥
func ComputeHmacSha512(message string, secret string) string {
	key := []byte(secret)
	h := hmac.New(sha512.New, key)
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
