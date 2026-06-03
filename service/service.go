package service

type ServiceType string

const (
	ServiceGithub ServiceType = "gh"
	ServiceGoods  ServiceType = "goods"
	ServiceTask   ServiceType = "task"
	ServiceBooks  ServiceType = "books"
	ServiceMovie  ServiceType = "movie"
	ServiceMusic  ServiceType = "music"
	ServiceDiary  ServiceType = "diary"
	ServiceNtl    ServiceType = "ntl"
)
