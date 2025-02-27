package service

type ServiceType string

const (
	ServiceBooks  ServiceType = "books"
	ServiceFc2    ServiceType = "fc2"
	ServiceGithub ServiceType = "gh"
	ServiceGoods  ServiceType = "goods"
	ServiceTask   ServiceType = "task"
	ServiceVideo  ServiceType = "video"
	ServiceWiki   ServiceType = "wiki"
)
