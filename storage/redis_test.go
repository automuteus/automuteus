package storage

import (
	"log"
	"testing"
)

func TestRedisDriver(t *testing.T) {
	r := &RedisCache{}
	err := r.Init(RedisParameters{
		Addr:     "192.168.1.8:6379",
		Username: "",
		Password: "",
	})
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	err = r.InitGuildSettings("141082723635691529", "mega")
	if err != nil {
		log.Println(err)
	}
}
