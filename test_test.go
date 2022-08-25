package mini_groupcache

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func Test(t *testing.T) {
	type Time time.Time
	type Log struct {
		Content   string    `json:"content"`
		CreatedAt time.Time `json:"created_at"`
	}
	
	log := Log{
		Content:   "Hello",
		CreatedAt: time.Now(),
	}

	bytes, err := json.Marshal(log)
	if err != nil {
		t.Fatal(err)
	}
	
	fmt.Println(string(bytes))
}
