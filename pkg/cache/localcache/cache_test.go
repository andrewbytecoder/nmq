// cache_test.go
package localcache

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"
)

func TestNewCache(t *testing.T) {
	// 测试创建新的缓存实例
	cache := NewCache()
	if cache.cache == nil {
		t.Error("Expected cache to be initialized")
	}

	if cache.member == nil {
		t.Error("Expected member map to be initialized")
	}

	if cache.capture == nil {
		t.Error("Expected capture function to be initialized")
	}
}

func TestSetAndGet(t *testing.T) {
	cache := NewCache()

	// 测试设置和获取缓存项
	key := "test_key"
	value := "test_value"

	cache.Set(key, value, 0) // 永不过期

	// 测试获取存在的缓存项
	if v, ok := cache.Get(key); !ok {
		t.Error("Expected key to exist")
	} else if v != value {
		t.Errorf("Expected value to be %v, got %v", value, v)
	}

	// 测试获取不存在的缓存项
	if _, ok := cache.Get("nonexistent"); ok {
		t.Error("Expected key to not exist")
	}
}

func TestSetWithExpire(t *testing.T) {
	cache := NewCache()

	key := "expiring_key"
	value := "expiring_value"

	// 设置1秒后过期的缓存项
	cache.Set(key, value, time.Second)

	// 立即获取应该成功
	if v, ok := cache.Get(key); !ok {
		t.Error("Expected key to exist before expiration")
	} else if v != value {
		t.Errorf("Expected value to be %v, got %v", value, v)
	}

	// 等待过期后获取应该失败
	time.Sleep(time.Second + time.Millisecond*100)

	if _, ok := cache.Get(key); ok {
		t.Error("Expected key to not exist after expiration")
	}
}

func TestAdd(t *testing.T) {
	cache := NewCache()

	key := "add_key"
	value := "add_value"

	// 测试添加新项
	if err := cache.Add(key, value, 0); err != nil {
		t.Errorf("Expected no error when adding new key, got %v", err)
	}

	// 测试重复添加应该返回错误
	if err := cache.Add(key, "another_value", 0); !errors.Is(err, CacheExist) {
		t.Errorf("Expected CacheExist error when adding existing key, got %v", err)
	}
}

func TestReplace(t *testing.T) {
	cache := NewCache()

	key := "replace_key"
	value := "replace_value"
	newValue := "new_value"

	// 测试替换不存在的项应该返回错误
	if err := cache.Replace(key, newValue, 0); !errors.Is(err, CacheNoExist) {
		t.Errorf("Expected CacheNoExist error when replacing non-existent key, got %v", err)
	}

	// 先添加项
	cache.Set(key, value, 0)

	// 测试替换存在的项
	if err := cache.Replace(key, newValue, 0); err != nil {
		t.Errorf("Expected no error when replacing existing key, got %v", err)
	}

	// 验证值已被替换
	if v, ok := cache.Get(key); !ok {
		t.Error("Expected key to exist after replacement")
	} else if v != newValue {
		t.Errorf("Expected value to be %v, got %v", newValue, v)
	}
}

func TestIncrementDecrement(t *testing.T) {
	cache := NewCache()

	// 测试对不存在的键进行增量操作
	if err := cache.Increment("nonexistent", 1); !errors.Is(err, CacheNoExist) {
		t.Errorf("Expected CacheNoExist error, got %v", err)
	}

	// 设置整数值
	key := "int_key"
	initialValue := 10
	cache.Set(key, initialValue, 0)

	// 测试增量操作
	if err := cache.Increment(key, 5); err != nil {
		t.Errorf("Expected no error when incrementing, got %v", err)
	}

	// 验证结果
	if v, ok := cache.Get(key); !ok {
		t.Error("Expected key to exist after increment")
	} else if v != 15 {
		t.Errorf("Expected value to be 15, got %v", v)
	}

	// 测试减量操作
	if err := cache.Decrement(key, 3); err != nil {
		t.Errorf("Expected no error when decrementing, got %v", err)
	}

	// 验证结果
	if v, ok := cache.Get(key); !ok {
		t.Error("Expected key to exist after decrement")
	} else if v != 12 {
		t.Errorf("Expected value to be 12, got %v", v)
	}
}

func TestIncrementDecrementFloat(t *testing.T) {
	cache := NewCache()

	// 测试对不存在的键进行浮点增量操作
	if err := cache.IncrementFloat("nonexistent", 1.5); !errors.Is(err, CacheNoExist) {
		t.Errorf("Expected CacheNoExist error, got %v", err)
	}

	// 设置浮点数值
	key := "float_key"
	initialValue := 10.5
	cache.Set(key, initialValue, 0)

	// 测试浮点增量操作
	if err := cache.IncrementFloat(key, 2.3); err != nil {
		t.Errorf("Expected no error when incrementing float, got %v", err)
	}

	// 验证结果
	if v, ok := cache.Get(key); !ok {
		t.Error("Expected key to exist after float increment")
	} else if v != 12.8 {
		t.Errorf("Expected value to be 12.8, got %v", v)
	}

	// 测试浮点减量操作
	if err := cache.DecrementFloat(key, 1.1); err != nil {
		t.Errorf("Expected no error when decrementing float, got %v", err)
	}

	// 验证结果
	if _, ok := cache.Get(key); !ok {
		t.Error("Expected key to exist after float decrement")
	}
}

func TestDelete(t *testing.T) {
	cache := NewCache()

	key := "delete_key"
	value := "delete_value"

	// 测试删除不存在的项
	cache.Delete(key)

	// 设置值并测试删除
	cache.Set(key, value, 0)

	if _, ok := cache.Get(key); !ok {
		t.Error("Expected key to exist before deletion")
	}

	cache.Delete(key)

	if _, ok := cache.Get(key); ok {
		t.Error("Expected key to not exist after deletion")
	}
}

func TestDeleteExpire(t *testing.T) {
	cache := NewCache()

	// 设置一个即将过期的项和一个永不过期的项
	cache.Set("expiring_key", "expiring_value", time.Millisecond*100)
	cache.Set("permanent_key", "permanent_value", 0)

	// 等待第一个项过期
	time.Sleep(time.Millisecond * 200)

	// 删除过期项
	cache.DeleteExpire()

	// 验证过期项已被删除，永久项仍然存在
	if _, ok := cache.Get("expiring_key"); ok {
		t.Error("Expected expiring key to be deleted")
	}

	if _, ok := cache.Get("permanent_key"); !ok {
		t.Error("Expected permanent key to still exist")
	}
}

func TestCount(t *testing.T) {
	cache := NewCache()

	// 初始计数应该为0
	if count := cache.Count(); count != 0 {
		t.Errorf("Expected count to be 0, got %d", count)
	}

	// 添加一些项
	cache.Set("key1", "value1", 0)
	cache.Set("key2", "value2", time.Second) // 即使过期也计入总数

	if count := cache.Count(); count != 2 {
		t.Errorf("Expected count to be 2, got %d", count)
	}
}

func TestIterator(t *testing.T) {
	cache := NewCache()

	// 添加有效项和即将过期的项
	cache.Set("valid_key", "valid_value", 0)
	cache.Set("expiring_key", "expiring_value", time.Millisecond*100)

	// 等待过期项过期
	time.Sleep(time.Millisecond * 200)

	// 获取迭代器
	iterator := cache.Iterator()

	// 应该只包含有效的项
	if len(iterator) != 1 {
		t.Errorf("Expected iterator length to be 1, got %d", len(iterator))
	}

	if _, ok := iterator["valid_key"]; !ok {
		t.Error("Expected valid_key to be in iterator")
	}

	if _, ok := iterator["expiring_key"]; ok {
		t.Error("Expected expiring_key to not be in iterator")
	}
}

func TestFlush(t *testing.T) {
	cache := NewCache()

	// 添加一些项
	cache.Set("key1", "value1", 0)
	cache.Set("key2", "value2", 0)

	if count := cache.Count(); count != 2 {
		t.Errorf("Expected count to be 2 before flush, got %d", count)
	}

	// 清空缓存
	cache.Flush()

	if count := cache.Count(); count != 0 {
		t.Errorf("Expected count to be 0 after flush, got %d", count)
	}
}

func TestSaveLoad(t *testing.T) {
	cache := NewCache()

	// 添加一些测试数据
	testData := map[string]interface{}{
		"string_key": "string_value",
		"int_key":    42,
		"float_key":  3.14,
	}

	for k, v := range testData {
		cache.Set(k, v, 0)
	}

	// 保存到缓冲区
	buf := &bytes.Buffer{}
	if err := cache.Save(buf); err != nil {
		t.Errorf("Expected no error when saving, got %v", err)
	}

	// 创建新缓存并从缓冲区加载
	newCache := NewCache()
	if err := newCache.Load(buf); err != nil {
		t.Errorf("Expected no error when loading, got %v", err)
	}

	// 验证数据是否正确加载
	for k, expected := range testData {
		if actual, ok := newCache.Get(k); !ok {
			t.Errorf("Expected key %s to exist after loading", k)
		} else if actual != expected {
			t.Errorf("Expected value for key %s to be %v, got %v", k, expected, actual)
		}
	}
}

func TestSaveLoadFile(t *testing.T) {
	cache := NewCache()

	// 添加测试数据
	cache.Set("file_test_key", "file_test_value", 0)

	// 创建临时文件
	tmpFile, err := ioutil.TempFile("", "cache_test_*.gob")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFileName := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpFileName)

	// 保存到文件
	if err := cache.SaveFile(tmpFileName); err != nil {
		t.Errorf("Expected no error when saving to file, got %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(tmpFileName); os.IsNotExist(err) {
		t.Error("Expected cache file to exist")
	}

	// 从文件加载
	newCache := NewCache()
	if err := newCache.LoadFile(tmpFileName); err != nil {
		t.Errorf("Expected no error when loading from file, got %v", err)
	}

	// 验证数据
	if value, ok := newCache.Get("file_test_key"); !ok {
		t.Error("Expected key to exist after loading from file")
	} else if value != "file_test_value" {
		t.Errorf("Expected value to be file_test_value, got %v", value)
	}
}

func TestConcurrentAccess(t *testing.T) {
	cache := NewCache()

	// 并发测试：多个goroutine同时访问缓存
	var wg sync.WaitGroup
	key := "concurrent_key"

	// 启动多个goroutine进行读写操作
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// 每个goroutine设置和获取不同的值
			testKey := key + string(rune(i))
			testValue := i

			cache.Set(testKey, testValue, 0)

			if value, ok := cache.Get(testKey); !ok {
				t.Errorf("Goroutine %d: Expected key %s to exist", i, testKey)
			} else if value != testValue {
				t.Errorf("Goroutine %d: Expected value for %s to be %d, got %v", i, testKey, testValue, value)
			}
		}(i)
	}

	wg.Wait()

	// 验证所有项都正确设置
	for i := 0; i < 10; i++ {
		testKey := key + string(rune(i))
		if _, ok := cache.Get(testKey); !ok {
			t.Errorf("Expected key %s to exist after concurrent operations", testKey)
		}
	}
}

func TestGetWithExpire(t *testing.T) {
	cache := NewCache()

	// 测试获取不存在的键
	if _, _, ok := cache.GetWithExpire("nonexistent"); ok {
		t.Error("Expected ok to be false for nonexistent key")
	}

	// 设置带过期时间的键
	key := "expiring_key"
	value := "expiring_value"
	duration := time.Second

	cache.Set(key, value, duration)

	// 获取值和过期时间
	if v, expireTime, ok := cache.GetWithExpire(key); !ok {
		t.Error("Expected ok to be true for existing key")
	} else {
		if v != value {
			t.Errorf("Expected value to be %v, got %v", value, v)
		}

		// 检查过期时间是否合理
		if expireTime.Before(time.Now()) {
			t.Error("Expected expire time to be in the future")
		}
	}

	// 设置永不过期的键
	permanentKey := "permanent_key"
	permanentValue := "permanent_value"

	cache.Set(permanentKey, permanentValue, 0)

	// 获取永不过期键的值和过期时间
	if v, expireTime, ok := cache.GetWithExpire(permanentKey); !ok {
		t.Error("Expected ok to be true for permanent key")
	} else {
		if v != permanentValue {
			t.Errorf("Expected value to be %v, got %v", permanentValue, v)
		}

		// 永不过期的键应该返回零时间
		if !expireTime.IsZero() {
			t.Error("Expected expire time to be zero for permanent key")
		}
	}
}
