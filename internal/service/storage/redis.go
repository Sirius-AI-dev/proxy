package storage

import (
	"github.com/go-redis/redis"
	"strconv"
	"time"
)

type redisStorage struct {
	redis *redis.Client
}

func (r *redisStorage) Get(key string) string {
	return r.redis.Get(key).Val()
}
func (r *redisStorage) Put(key string, value string, expired time.Duration) {
	r.redis.Set(key, value, expired)
}
func (r *redisStorage) Del(key ...string) {
	r.redis.Del(key...)
}

func NewRedisStorage(host string, password string, database string) Storage {
	db, err := strconv.Atoi(database)
	if err != nil {
		panic(err)
	}

	return &redisStorage{
		redis: redis.NewClient(&redis.Options{
			Addr:     host,
			Password: password,
			DB:       db,
		}),
	}
}
