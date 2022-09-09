package utils

import (
	"encoding/json"
	"math/rand"

	"github.com/sirupsen/logrus"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func NoErrorFieldInJSON(jsonStr string) bool {
	var tmp map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &tmp)

	if err != nil {
		logrus.Warnf("decode json string failed, %s, %v\n", jsonStr, err)
		return false
	}

	if tmp["error"] == nil {
		return true
	}

	return false
}

func IsBatch(msg []byte) bool {
	for _, c := range msg {
		if c == 0x20 || c == 0x09 || c == 0x0a || c == 0x0d {
			continue
		}
		return c == '['
	}
	return false
}
