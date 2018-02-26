package controller

import (
	"error"
	"time"
)

type FakeStorage struct{}

func NewFakeStorage() *FakeStorage {
	return &FakeStorage{}
}

func (f *FakeStorage) Set(key string, value string, expiration time.Duration) error {
	return nil
}

func (f *FakeStorage) Get(key string) (string, error) {
	return "fake", nil
}

func (f *FakeStorage) Del(key string) error {
	return nil
}
