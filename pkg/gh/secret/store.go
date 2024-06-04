package secret

import (
	aw "github.com/deanishe/awgo"
)

// Store describe a store to manage secret values.
type Store struct {
	wf *aw.Workflow
}

// func accountKey(key string) (string, error) {
// 	me, err := user.Current()
// 	if err != nil {
// 		return "", err
// 	}
//
// 	return fmt.Sprintf("%s-%s", me.Username, key), nil
// }

// NewStore return an instance of Store
func NewStore(wf *aw.Workflow) *Store {
	return &Store{
		wf: wf,
	}
}

// // Store save the secret
// func (s *Store) Store(key string, secret string) error {
// 	myKey, err := accountKey(key)
// 	if err != nil {
// 		return err
// 	}
//
// 	return s.wf.Keychain.Set(myKey, secret)
// }

// Get fetch the value from the secret store.
// func (s *Store) Get(key string) (string, error) {
// 	myKey, err := accountKey(key)
// 	if err != nil {
// 		switch err {
// 		case keychain.ErrNotFound:
// 			return "", nil
// 		default:
// 			return "", err
// 		}
// 	}
//
// 	return s.wf.Keychain.Get(myKey)
// }

// // GetAPIToken fetch the token string from this store.
// func (s *Store) GetAPIToken() (string, error) {
// 	return s.Get(KeyGithubAPIToken)
// }

// // Delete removes the value from the secret store.
// func (s *Store) Delete(key string) error {
// 	myKey, err := accountKey(key)
// 	if err != nil {
// 		return err
// 	}
//
// 	return s.wf.Keychain.Delete(myKey)
// }
