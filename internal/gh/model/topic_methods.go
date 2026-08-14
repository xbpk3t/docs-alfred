package model

// DirName returns the directory name implied by a topic.
func (t *Topic) DirName() string {
	return t.Topic
}
