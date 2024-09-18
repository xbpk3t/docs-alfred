package gh

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	GhURL = "https://github.com/"
)

type ConfigRepos []ConfigRepo

type ConfigRepo struct {
	Type  string `yaml:"type"`
	Repos `yaml:"repo"`
	Qs    Qs `yaml:"qs,omitempty"`
}

type Repository struct {
	LastUpdated time.Time
	Doc         string `yaml:"doc,omitempty"` // 该repo的官方文档URL
	URL         string `yaml:"url"`           // 该repo的gh URL
	Name        string `yaml:"name,omitempty"`
	User        string
	Des         string `yaml:"des,omitempty"` // 描述
	Type        string `yaml:"type"`          // used to mark Type

	Tag string `yaml:"tag,omitempty"` // 原本的文件名，比如说 db.yml, db.yml, ...
	Qs  Qs     `yaml:"qs,omitempty"`
	Cmd Cmd    `yaml:"cmd,omitempty"`

	Qq  `yaml:"qq,omitempty"`
	Sub Repos `yaml:"sub,omitempty"` // 用来标识属于该repo的一些repo
	Dep Repos `yaml:"dep,omitempty"` // 用来标识可以被改repo替代的一些repo

	IsStar bool // 用来标识该repo是否在gh.yml中

	// Alias       string `yaml:"alias,omitempty"` // 如果有alias，则直接渲染为[alias](URL)，而不是[User/Name](URL)

	// Pix         []string `yaml:"pix"`
	// Use         []struct {
	// 	URL string `yaml:"url,omitempty"`
	// 	Des string `yaml:"des,omitempty"`
	// } `yaml:"use,omitempty"`
}

type Qq []struct {
	Topic string       `yaml:"topic"`
	URL   string       `yaml:"url,omitempty"`
	Des   string       `yaml:"des,omitempty"`
	Qs    Qs           `yaml:"qs,omitempty"`
	Sub   []Repository `yaml:"sub,omitempty"` // 用来标识属于该repo的一些repo
}

type Qs []Qt

type Qt struct {
	Q string   `yaml:"q,omitempty"` // 问题
	X string   `yaml:"x,omitempty"` // 简要回答
	P string   `yaml:"p,omitempty"`
	U string   `yaml:"u,omitempty"` // url
	S []string `yaml:"s,omitempty"` // 该问题的一些发散问题
}

type Cmd []struct {
	C string `yaml:"c"`
	X string `yaml:"x,omitempty"` // 该命令的描述
	K bool   `yaml:"k,omitempty"` // 该命令是否重要 default: false
}

type Repos []Repository

type Gh []string

func NewConfigRepos(f []byte) ConfigRepos {
	var ghs ConfigRepos

	d := yaml.NewDecoder(bytes.NewReader(f))
	for {
		// create new spec here
		spec := new(ConfigRepos)
		// pass a reference to spec reference
		if err := d.Decode(&spec); err != nil {
			// break the loop in case of EOF
			if errors.Is(err, io.EOF) {
				break
			}
			panic(err)
		}

		ghs = append(ghs, *spec...)
	}
	return ghs
}

// 给Repo的Tag字段赋值，否则为空字符串
func (cr ConfigRepos) WithTag(tag string) ConfigRepos {
	for _, repo := range cr {
		for i := range repo.Repos {
			repo.Repos[i].Tag = tag
		}
	}
	return cr
}

// MergeConfigs 将多个 ConfigRepos 合并为一个
// func MergeConfigs(in <-chan ConfigRepos) ConfigRepos {
// 	merged := make(ConfigRepos)
//
// 	for repo := range in {
// 		for k, v := range repo {
// 			merged[k] = v
// 		}
// 	}
//
// 	return merged
// }

// ToRepos Convert Type to Repo
func (cr ConfigRepos) ToRepos() Repos {
	repos := make(Repos, 0)
	// for _, config := range cr {
	// 	rec(config.Repos, config.Type, &repos)
	// }
	for _, config := range cr {
		for _, repo := range config.Repos {
			repos = append(repos, processRepo(repo, config.Type)...)
		}
	}
	return repos
}

// func rec(rs Repos, t string, allRepos *Repos) {
// 	repos := make(Repos, 0)
//
// 	for _, repo := range rs {
// 		if strings.Contains(repo.URL, GhURL) {
// 			sx, found := strings.CutPrefix(repo.URL, GhURL)
// 			if found {
// 				splits := strings.Split(sx, "/")
// 				if len(splits) == 2 {
// 					repo.User = splits[0]
// 					repo.Name = splits[1]
// 					repo.IsStar = true
// 					// repo.Type = config.Type
// 					repo.Type = t
// 					repos = append(repos, repo)
//
//
// 				} else {
// 					log.Printf("URL Split Error: unexpected format: %s", repo.URL)
// 				}
// 			} else {
// 				log.Printf("CutPrefix Error URL: %s", repo.URL)
// 			}
// 		} else {
// 			log.Printf("URL Not Contains: %s", repo.URL)
// 		}
//
// 		// 递归处理 Sub 字段
// 		rec(repo.Sub, repo.Type, allRepos)
//
// 		// 递归处理 Dep 字段
// 		rec(repo.Dep, repo.Type, allRepos)
//
// 		allRepos = append(allRepos, repos...)
// 	}
// 	// return repos
// }

// // processRepo 处理单个仓库，包括递归处理子仓库和依赖仓库
// func processRepo(repo Repository, configType string) Repos {
// 	repos := make(Repos, 0)
// 	if strings.Contains(repo.URL, GhURL) {
// 		sx, found := strings.CutPrefix(repo.URL, GhURL)
// 		if found {
// 			splits := strings.Split(sx, "/")
// 			if len(splits) == 2 {
// 				repo.User = splits[0]
// 				repo.Name = splits[1]
// 				repo.IsStar = true
// 				repo.Type = configType
// 				repos = append(repos, repo)
// 				// 递归处理子仓库
// 				repos = append(repos, processSubRepos(repo.Sub, configType)...)
// 				// 递归处理依赖仓库
// 				repos = append(repos, processDepRepos(repo.Dep, configType)...)
// 			} else {
// 				log.Printf("URL Split Error: unexpected format: %s", repo.URL)
// 			}
// 		} else {
// 			log.Printf("CutPrefix Error URL: %s", repo.URL)
// 		}
// 	} else {
// 		log.Printf("URL Not Contains: %s", repo.URL)
// 	}
// 	return repos
// }
//
// // processSubRepos 递归处理子仓库
// func processSubRepos(subRepos []Repository, configType string) Repos {
// 	repos := make(Repos, 0)
// 	for _, subRepo := range subRepos {
// 		repos = append(repos, processRepo(subRepo, configType)...)
// 	}
// 	return repos
// }
//
// // processDepRepos 递归处理依赖仓库
// func processDepRepos(depRepos []Repository, configType string) Repos {
// 	repos := make(Repos, 0)
// 	for _, depRepo := range depRepos {
// 		repos = append(repos, processRepo(depRepo, configType)...)
// 	}
// 	return repos
// }

// processRepo 递归处理单个仓库
func processRepo(repo Repository, configType string) Repos {
	repos := make(Repos, 0)
	if strings.Contains(repo.URL, GhURL) {
		sx, found := strings.CutPrefix(repo.URL, GhURL)
		if found {
			splits := strings.Split(sx, "/")
			if len(splits) == 2 {
				repo.User = splits[0]
				repo.Name = splits[1]
				repo.IsStar = true
				repo.Type = configType
				repos = append(repos, repo)
			} else {
				log.Printf("URL Split Error: unexpected format: %s", repo.URL)
			}
		} else {
			log.Printf("CutPrefix Error URL: %s", repo.URL)
		}
	} else {
		log.Printf("URL Not Contains: %s", repo.URL)
	}
	for _, subRepo := range repo.Sub {
		repos = append(repos, processRepo(subRepo, fmt.Sprintf("%s [SUB: %s]", configType, repo.FullName()))...)
	}
	for _, depRepo := range repo.Dep {
		repos = append(repos, processRepo(depRepo, fmt.Sprintf("%s [DEP: %s]", configType, repo.FullName()))...)
	}
	return repos
}

//
// // 为 Repository 类型添加 ToRepos 方法，以便递归调用
// func (repo Repository) ToRepos() Repos {
// 	repos := make(Repos, 0)
// 	// 处理当前仓库
// 	// ...（这里可以添加处理当前仓库的逻辑，如果需要的话）
// 	// 递归处理子仓库
// 	if len(repo.Sub) > 0 {
// 		for _, subRepo := range repo.Sub {
// 			repos = append(repos, subRepo.ToRepos()...)
// 		}
// 	}
// 	// 递归处理依赖仓库
// 	if len(repo.Dep) > 0 {
// 		for _, depRepo := range repo.Dep {
// 			repos = append(repos, depRepo.ToRepos()...)
// 		}
// 	}
// 	return repos
// }
//
// func (rs Repos) ToRepos() Repos {
// 	result := make(Repos, 0)
// 	for _, repo := range rs {
// 		// 添加当前仓库
// 		result = append(result, repo)
//
// 		// 递归处理子仓库
// 		for _, subRepo := range repo.Sub {
// 			result = append(result, subRepo.ToRepos()...)
// 		}
//
// 		// 递归处理依赖仓库
// 		for _, depRepo := range repo.Dep {
// 			result = append(result, depRepo.ToRepos()...)
// 		}
// 	}
// 	return result
// }

// ExtractTags Extract tags from Repos
func (rs Repos) ExtractTags() []string {
	var tags []string
	for _, repo := range rs {
		if repo.Type != "" && !slices.Contains(tags, repo.Type) {
			tags = append(tags, repo.Type)
		}
	}
	return tags
}

// QueryReposByTag Query Repos by Type
func (rs Repos) QueryReposByTag(tag string) Repos {
	var res Repos
	for _, repo := range rs {
		if repo.Type == tag {
			res = append(res, repo)
		}
	}
	return res
}

// FilterReposMD x
// func (cr *ConfigRepos) FilterReposMD() ConfigRepos {
// 	var filteredConfig ConfigRepos
// 	for _, crv := range *cr {
// 		// if crv.Md {
// 		var filteredRepos []Repository
// 		for _, repo := range crv.Repos {
// 			if repo.Qs != nil {
// 				// repo.Pix = addMarkdownPicFormat(repo.Pix)
// 				filteredRepos = append(filteredRepos, repo)
// 			}
// 		}
// 		crv.Repos = filteredRepos
// 		filteredConfig = append(filteredConfig, crv)
// 		// }
// 	}
// 	return filteredConfig
// }

func (cr *ConfigRepos) FilterReposMD() ConfigRepos {
	var filteredConfig ConfigRepos
	for _, crv := range *cr {
		// if crv.Md {
		var filteredRepos []Repository
		for _, repo := range crv.Repos {
			if repo.Qs != nil {
				// repo.Pix = addMarkdownPicFormat(repo.Pix)
				filteredRepos = append(filteredRepos, repo)
			}
		}
		crv.Repos = filteredRepos
		filteredConfig = append(filteredConfig, crv)
		// }
	}
	return filteredConfig
}

// IsTypeQsEmpty 判断该type是否为空
func (cr *ConfigRepos) IsTypeQsEmpty() bool {
	for _, crv := range *cr {
		if crv.Qs == nil {
			return true
		}
	}
	return false
}

// func (cr *ConfigRepos) FilterWorksMD() ConfigRepos {
// 	var filteredConfig ConfigRepos
// 	for _, crv := range *cr {
// 		// if crv.Md {
// 		var filteredRepos []Repository
// 		crv.Repos = filteredRepos
// 		filteredConfig = append(filteredConfig, crv)
// 		// }
// 	}
// 	return filteredConfig
// }

// GetFileNameFromURL 从给定的 URL 中提取并返回文件名。
// func GetFileNameFromURL(urlString string) (string, error) {
// 	// 解析 URL
// 	parsedURL, err := url.Parse(urlString)
// 	if err != nil {
// 		return "", fmt.Errorf("error parsing URL: %v", err)
// 	}
//
// 	// 获取路径
// 	urlPath := parsedURL.Path
//
// 	// 获取文件名
// 	fileName := path.Base(urlPath)
//
// 	return fileName, nil
// }

// 用来渲染pic
// func addMarkdownPicFormat(URLs []string) []string {
// 	res := make([]string, len(URLs))
// 	for _, u := range URLs {
// 		name, _ := GetFileNameFromURL(u)
// 		res = append(res, fmt.Sprintf("![%s](%s)\n", name, u))
// 	}
// 	return res
// }
