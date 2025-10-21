// iterator_test.go
package localcache

import (
	"testing"
	"time"
)

func TestIteratorExpired(t *testing.T) {
	// 测试永不过期的缓存项
	t.Run("NoExpire", func(t *testing.T) {
		iterator := &Iterator{
			Val:    "test",
			Expire: 0, // 永不过期
		}

		if iterator.Expired() {
			t.Error("Expected iterator with expire=0 to never expire")
		}
	})

	// 测试已过期的缓存项
	t.Run("Expired", func(t *testing.T) {
		iterator := &Iterator{
			Val:    "test",
			Expire: time.Now().Add(-1 * time.Hour).UnixNano(), // 1小时前过期
		}

		if !iterator.Expired() {
			t.Error("Expected iterator with past expire time to be expired")
		}
	})

	// 测试未过期的缓存项
	t.Run("NotExpired", func(t *testing.T) {
		iterator := &Iterator{
			Val:    "test",
			Expire: time.Now().Add(1 * time.Hour).UnixNano(), // 1小时后过期
		}

		if iterator.Expired() {
			t.Error("Expected iterator with future expire time to not be expired")
		}
	})

	// 测试使用自定义时间判断过期
	t.Run("ExpiredWithCustomTime", func(t *testing.T) {
		futureTime := time.Now().Add(2 * time.Hour).UnixNano()
		pastTime := time.Now().Add(-2 * time.Hour).UnixNano()

		iterator := &Iterator{
			Val:    "test",
			Expire: time.Now().Add(1 * time.Hour).UnixNano(), // 1小时后过期
		}

		// 使用未来的自定义时间判断，应该未过期
		if iterator.Expired(pastTime) {
			t.Error("Expected iterator to not be expired when comparing with past time")
		}

		// 使用过去的自定义时间判断，应该已过期
		if !iterator.Expired(futureTime) {
			t.Error("Expected iterator to be expired when comparing with future time")
		}
	})
}

func TestIteratorFields(t *testing.T) {
	// 测试 Iterator 结构体字段
	testVal := "test value"
	expireTime := time.Now().Add(1 * time.Hour).UnixNano()

	iterator := &Iterator{
		Val:    testVal,
		Expire: expireTime,
	}

	if iterator.Val != testVal {
		t.Errorf("Expected Val to be %v, got %v", testVal, iterator.Val)
	}

	if iterator.Expire != expireTime {
		t.Errorf("Expected Expire to be %v, got %v", expireTime, iterator.Expire)
	}
}

func TestKvStruct(t *testing.T) {
	// 测试 kv 结构体字段
	testKey := "test_key"
	testValue := "test_value"

	kvItem := kv{
		key:   testKey,
		value: testValue,
	}

	if kvItem.key != testKey {
		t.Errorf("Expected key to be %v, got %v", testKey, kvItem.key)
	}

	if kvItem.value != testValue {
		t.Errorf("Expected value to be %v, got %v", testValue, kvItem.value)
	}
}
