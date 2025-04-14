package common

import (
	"log"
	"os"
)

type UtilsInterface interface {
	Exists(path string) (bool, error)
	FailOnError(err error, msg string)
	TouchFile(name string) error
}

type Utils struct{}

func (u *Utils) Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (u *Utils) FailOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err.Error())
	}
}

func (u *Utils) TouchFile(name string) error {
	file, err := os.OpenFile(name, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	return file.Close()
}
