package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-redis/redis"
	"github.com/golang/glog"
)

type Storage interface {
	Set(key string, value string, expiration time.Duration) error
	Get(key string) (value string, err error)
	Del(key string) error
}

type PreMarshal interface {
	PreMarshal() error
}

type PostUnmarshal interface {
	PostUnmarshal() error
}

/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////

// TODO redis handles concurrency?
type RedisStorage struct {
	Redis *redis.Client
}

func NewRedisStorage(address, password string, database int) *RedisStorage {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
		DB:       database,
	})
	return &RedisStorage{redisClient}
}

func (r *RedisStorage) Set(key string, value string, expiration time.Duration) error {
	return r.Redis.Set(key, value, expiration).Err()
}

func (r *RedisStorage) Get(key string) (string, error) {
	v, e := r.Redis.Get(key).Result()
	return v, e
}

func (r *RedisStorage) Del(key string) error {
	return r.Redis.Del(key).Err()
}

/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////

// TODO handle concurrent access?
type MemStorage struct {
	storage map[string]string
}

func NewMemStorage() *MemStorage {
	return &MemStorage{storage: make(map[string]string, 0)}
}

func (m *MemStorage) Set(key string, value string, expiration time.Duration) error {
	glog.Infof("Saving: <%s> to key: <%s>\n", value, key)
	m.storage[key] = value
	return nil
}

func (m *MemStorage) Get(key string) (value string, err error) {
	if v, ok := m.storage[key]; ok {
		return v, nil
	} else {
		return "", errors.New("nothing under that key")
	}
}

func (m *MemStorage) Del(key string) error {
	delete(m.storage, key)
	return nil
}

/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////

func (binding *Binding) PreMarshal() error {

	//	if err := binding.Instance.PreMarshal(); err != nil {
	//		glog.Errorf("Failed to sync binding.Instance: %s", err)
	//		return err
	//	}
	return nil
}

func (binding *Binding) PostUnmarshal() error {

	//	if err := binding.Instance.PostUnmarshal(); err != nil {
	//		glog.Errorf("Failed to unmarshal Binding.Instance: %s", err)
	//		return err
	//	}

	return nil
}

/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////

func instanceName(id string) string {
	return fmt.Sprintf("instance-%s", id)
}

func InstanceExists(s Storage, id string) bool {
	if _, err := s.Get(instanceName(id)); err == nil {
		return true
	}
	return false
}

func LoadInstance(s Storage, id string) (*Instance, error) {

	if js, err := s.Get(instanceName(id)); err == nil {
		i := Instance{}
		if err := json.Unmarshal([]byte(js), &i); err == nil {
			return &i, nil
		} else {
			glog.Errorf("Error unmarshaling Instance: %s", err)
			return nil, err
		}
	} else {
		glog.Errorf("Error loading Instance: %s", err)
		return nil, err
	}
}

func SaveInstance(s Storage, id string, instance *Instance) error {

	if js, err := json.Marshal(instance); err == nil {
		if err := s.Set(instanceName(id), string(js[:]), 0); err != nil {
			glog.Errorf("Failed to save Instance: %s", err)
			return err
		}
	} else {
		glog.Errorf("Failed to marshal Instance: %s", err)
		return err
	}
	return nil
}

func DeleteInstance(s Storage, id string) error {
	if err := s.Del(instanceName(id)); err != nil {
		glog.Errorf("Failed to delete instance: %s", err)
		return err
	}
	return nil
}

/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////
/////////////////////////////////////////////////////////////////

func bindingName(id string) string {
	return fmt.Sprintf("binding-%s", id)
}

func BindingExists(s Storage, id string) bool {
	if _, err := s.Get(bindingName(id)); err == nil {
		return true
	}
	return false
}

func LoadBinding(s Storage, id string) (*Binding, error) {

	if js, err := s.Get(bindingName(id)); err == nil {
		b := Binding{}
		if err := json.Unmarshal([]byte(js), &b); err == nil {
			if err := b.PostUnmarshal(); err == nil {
				return &b, nil
			} else {
				glog.Errorf("Error Binding PostUnmarshal: %s", err)
				return nil, err
			}
		} else {
			glog.Errorf("Error unmarshaling Binding: %s", err)
			return nil, err
		}
	} else {
		glog.Errorf("Error loading Binding: %s", err)
		return nil, err
	}
}

func SaveBinding(s Storage, id string, binding *Binding) error {

	if err := binding.PreMarshal(); err != nil {
		glog.Errorf("Failed SaveBinding PreMarshal: %s", err)
		return err
	}
	if js, err := json.Marshal(binding); err == nil {
		if err := s.Set(bindingName(id), string(js[:]), 0); err != nil {
			glog.Errorf("Failed to save Binding: %s", err)
			return err
		}
	} else {
		glog.Errorf("Failed to marshal Binding: %s", err)
		return err
	}
	return nil
}

func DeleteBinding(s Storage, id string) error {
	if err := s.Del(bindingName(id)); err != nil {
		glog.Errorf("Failed to delete Binding: %s", err)
		return err
	}
	return nil
}
