package models

import (
	"github.com/jinzhu/gorm"
	"github.com/snowlyg/EasyDarwin/extend/db"
	"strconv"
)

type Stream struct {
	gorm.Model
	Status            bool
	PusherId          string `gorm:"type:varchar(256);unique"`
	URL               string `gorm:"type:varchar(256);unique"`
	RealURl           string `gorm:"type:varchar(256)"`
	CustomPath        string `gorm:"type:varchar(256)"`
	TransType         string `gorm:"type:varchar(256)"`
	IdleTimeout       int
	HeartbeatInterval int
}

func GetStream(formId string) *Stream {
	id, _ := strconv.ParseUint(formId, 10, 64)
	stream := Stream{}
	db.SQLite.Where("id = ?", id).First(&stream)
	return &stream
}
