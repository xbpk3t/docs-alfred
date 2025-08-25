# pwgen-alfred




## 开发过程



- [mrjooz/password-generator: 密码生成工具](https://github.com/mrjooz/password-generator)
- [密码生成工具](https://mrjooz.github.io/password-generator/)

但是看了一下，他的这个工具没有用 js 的标准库 sha512，而是用 [emn178/js-sha512](https://github.com/emn178/js-sha512) 这个库实现的，那么用 golang 就很难搞了。所以我直接用 golang 内置的 sha512 实现，导致最终的秘串和这个工具生成的不同。

goja 貌似可以直接加载整个 js 文件 [用 Golang 运行 JavaScript - 掘金](https://juejin.cn/post/6844904002975432717)，但是看了一下貌似很麻烦，所以就不想搞了。





## 使用中存在的问题


截止目前，几乎所有网站的密码都已经用这个工具修改过一遍了

可以说非常好用，但是

- 无法跨端使用
- 无法处理 2FA 认证的场景
- 密码复杂度问题，目前的密码只有小写字母、大写字母和数字三种，在某些验证规则严格的情况下，无法使用


---

跨端使用的问题，可以用 workflow+shortcut 解决，workflow 直接调用 shortcut，ios 上直接使用 shortcut 生成密码

但是 shortcut 只能调用 js 代码，需要把目前的 golang 代码再转成 js

到时候再说
