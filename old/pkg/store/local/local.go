package local

import (
	"github.com/zalando/go-keyring"
)

type LocalCred struct {
	AppName string
}

func NewLocalCred(AppName string) *LocalCred {
	return &LocalCred{
		AppName: AppName,
	}
}

func (l *LocalCred) Get(key string) (string, error) {
	return keyring.Get(l.AppName, key)
}

func (l *LocalCred) Set(key, value string) error {
	return keyring.Set(l.AppName, key, value)
}

func (l *LocalCred) Remove(key string) error {
	return keyring.Delete(l.AppName, key)
}
