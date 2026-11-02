package belajar_golang_redis

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// TOPIC 6: REDIS HASH FUNDAMENTALS
// ============================================================================
// Fundamental:
// Redis Hash adalah struktur data key-value di dalam key (dictionary/map).
// Sangat mirip dengan representasi `struct` di Golang atau objek JSON.
//
// Keunggulan Efisiensi Memori (Memory Optimization):
// Jika Anda memiliki 1 juta user dengan field name, email, age:
// - Menyimpan sebagai key terpisah: `user:1:name`, `user:1:email` sangat boros RAM
//   karena setiap key di Redis memiliki metadata overhead (pointer, robj, dict entry).
// - Menyimpan sebagai Hash: `user:1` -> {name: "...", email: "..."} sangat hemat RAM!
//   Di bawah batas `hash-max-listpack-entries` (default 512 field), Redis menyimpannya
//   sebagai array byte padat (`listpack`), memangkas penggunaan memori hingga 80%!
//
// Use Cases Populer:
// 1. Profil Pengguna (User Profile / Account State).
// 2. Shopping Cart (Keranjang belanja: ProductID -> Qty).
// 3. Konfigurasi dinamis aplikasi (Feature Flags).
// ============================================================================

type UserProfile struct {
	ID        string `redis:"id"`
	Username  string `redis:"username"`
	Email     string `redis:"email"`
	LoginHits int    `redis:"login_hits"`
}

func TestHashBasicOperations(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	userKey := UniqueKey("user:profile:1001")
	CleanKey(t, client, ctx, userKey)

	// 1. HSET: Menyimpan beberapa field sekaligus ke dalam hash
	// Format: HSET key field value [field value ...]
	err := client.HSet(ctx, userKey, map[string]interface{}{
		"id":         "1001",
		"username":   "adit_dev",
		"email":      "adit@example.com",
		"login_hits": 1,
	}).Err()
	assert.NoError(t, err)

	// 2. HGET: Mengambil nilai dari field tertentu -> O(1)
	email, err := client.HGet(ctx, userKey, "email").Result()
	assert.NoError(t, err)
	assert.Equal(t, "adit@example.com", email)

	// 3. HEXISTS: Memeriksa apakah suatu field ada di dalam hash -> O(1)
	exists, err := client.HExists(ctx, userKey, "username").Result()
	assert.NoError(t, err)
	assert.True(t, exists)

	notExists, err := client.HExists(ctx, userKey, "password_hash").Result()
	assert.NoError(t, err)
	assert.False(t, notExists)

	// 4. HINCRBY: Menambah nilai field angka secara atomik di dalam hash
	// Setiap kali user login, kita increment login_hits + 1
	newHits, err := client.HIncrBy(ctx, userKey, "login_hits", 1).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), newHits)

	// 5. HMGET: Mengambil subset beberapa field tertentu sekaligus
	subset, err := client.HMGet(ctx, userKey, "username", "login_hits").Result()
	assert.NoError(t, err)
	assert.Equal(t, "adit_dev", subset[0])
	assert.Equal(t, "2", subset[1])

	// 6. HGETALL: Mengambil SELURUH field dan value ke dalam map Go
	allData, err := client.HGetAll(ctx, userKey).Result()
	assert.NoError(t, err)
	assert.Equal(t, "1001", allData["id"])
	assert.Equal(t, "adit_dev", allData["username"])
	assert.Equal(t, "adit@example.com", allData["email"])
	assert.Equal(t, "2", allData["login_hits"])

	// 7. HDEL: Menghapus field tertentu di dalam hash
	deletedCount, err := client.HDel(ctx, userKey, "email").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), deletedCount)

	// 8. HLEN: Menghitung berapa field tersisa
	remainingFields, _ := client.HLen(ctx, userKey).Result()
	assert.Equal(t, int64(3), remainingFields, "Tersisa id, username, dan login_hits")
}

func TestHashScanStruct(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	userKey := UniqueKey("user:struct:2002")
	CleanKey(t, client, ctx, userKey)

	// Fundamental HScan / Scan into Go Struct:
	// Library go-redis memiliki fitur canggih `Scan` yang otomatis memetakan
	// field-field di Redis Hash ke dalam struct Go berdasarkan tag `redis:"field_name"`.

	// Simpan data
	_ = client.HSet(ctx, userKey, map[string]interface{}{
		"id":         "2002",
		"username":   "budi_backend",
		"email":      "budi@example.com",
		"login_hits": 5,
	}).Err()

	// Ambil dan unmarshal langsung ke struct
	var profile UserProfile
	err := client.HGetAll(ctx, userKey).Scan(&profile)
	assert.NoError(t, err)

	assert.Equal(t, "2002", profile.ID)
	assert.Equal(t, "budi_backend", profile.Username)
	assert.Equal(t, "budi@example.com", profile.Email)
	assert.Equal(t, 5, profile.LoginHits)
}
