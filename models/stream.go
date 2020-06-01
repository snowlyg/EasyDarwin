package models

import "github.com/jinzhu/gorm"

type Stream struct {
	gorm.Model
	URL               string `gorm:"type:varchar(256);unique"`
	StreamId          string `gorm:"type:varchar(256)"`
	CustomPath        string `gorm:"type:varchar(256)"`
	TransType         string `gorm:"type:varchar(256)"`
	IdleTimeout       int
	HeartbeatInterval int
	Status            bool
}
