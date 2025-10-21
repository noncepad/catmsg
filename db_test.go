package catmsg_test

import (
	"testing"

	"github.com/noncepad/catmsg"
)

func TestDatabase(t *testing.T) {
	db := catmsg.NewKVStore(100)
	mCheck := make(map[string]string)
	mCheck["key1"] = "value1"
	mCheck["key2"] = "value2"
	mCheck["key3"] = "value3"
	for k, v := range mCheck {
		err := db.Put([]byte(k), []byte(v))
		if err != nil {
			t.Fatal(err)
		}
	}
	for k, checkV := range mCheck {
		v, present := db.Get([]byte(k))
		if !present {
			t.Fatalf("key %s missing", k)
		}
		v1 := string(v)
		if v1 != checkV {
			t.Fatalf("database fail: %s %s %s", k, v1, checkV)
		}
	}
}
