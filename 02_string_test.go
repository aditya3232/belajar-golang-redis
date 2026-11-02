package belajar_golang_redis

import (
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// TOPIC 2: REDIS STRING FUNDAMENTALS
// ============================================================================
// Fundamental:
// Redis String adalah tipe data paling mendasar dan fleksibel.
// String di Redis diimplementasikan menggunakan SDS (Simple Dynamic String)
// yang bersifat Binary-Safe (bisa menyimpan teks biasa, JSON, HTML, bahkan file biner
// seperti gambar atau protobuf) hingga ukuran maksimal 512 MB per key.
//
// Kompleksitas Waktu:
// - SET, GET, INCR, DECR, SETNX: O(1) - sangat cepat (sub-millisecond).
// - MGET, MSET: O(N) di mana N adalah jumlah key yang diambil/disimpan.
//
// Use Cases Populer:
// 1. Caching respon API / JSON data.
// 2. Session storage pengguna website.
// 3. Counter atomik (view count, hit counter, rate limiting token bucket).
// 4. Distributed Lock sederhana menggunakan SETNX dengan TTL.
// ============================================================================

func TestStringSetAndGet(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	key := UniqueKey("string:basic")
	CleanKey(t, client, ctx, key)

	// 1. SET key value dengan TTL 0 (tanpa kedaluwarsa)
	// Di balik layar: Redis membuat key di memori dan mengasosiasikannya dengan SDS value.
	err := client.Set(ctx, key, "Halo Pengembang Golang!", 0).Err()
	assert.NoError(t, err)

	// 2. GET value dari key
	val, err := client.Get(ctx, key).Result()
	assert.NoError(t, err)
	assert.Equal(t, "Halo Pengembang Golang!", val)
}

func TestStringKeyNotFound(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	// Fundamental Handling Key Not Found:
	// Ketika key tidak ditemukan di Redis, library go-redis TIDAK mengembalikan nilai kosong "",
	// melainkan mengembalikan error khusus bernama `redis.Nil`.
	// Ini sangat penting untuk membedakan antara "key memang tidak ada" dengan
	// "key ada tetapi nilainya string kosong".
	nonExistentKey := UniqueKey("string:notfound")

	val, err := client.Get(ctx, nonExistentKey).Result()
	assert.Equal(t, redis.Nil, err, "Error harus sama persis dengan redis.Nil jika key tidak ada")
	assert.Empty(t, val)
}

func TestStringExpirationTTL(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	key := UniqueKey("string:ttl")
	CleanKey(t, client, ctx, key)

	// Fundamental Expiration (TTL - Time to Live):
	// Digunakan dalam pattern Cache-Aside untuk mencegah data stale selamanya di memori.
	// Redis memiliki dua mekanisme pembersihan key kedaluwarsa:
	// 1. Passive / Lazy: Dihapus saat key tersebut diakses (GET) dan dideteksi sudah expire.
	// 2. Active: Redis secara periodik mengambil sampel key acak dan menghapus yang sudah expire.
	ttl := 2 * time.Second
	err := client.SetEx(ctx, key, "Data Sementara", ttl).Err()
	assert.NoError(t, err)

	// Cek sisa waktu TTL dengan TTL()
	remainingTTL, err := client.TTL(ctx, key).Result()
	assert.NoError(t, err)
	assert.True(t, remainingTTL > 0 && remainingTTL <= ttl, "Sisa TTL harus positif dan <= 2 detik")

	// Tunggu sampai key kedaluwarsa
	time.Sleep(2100 * time.Millisecond)

	// Setelah lewat batas waktu, key otomatis musnah
	_, err = client.Get(ctx, key).Result()
	assert.Equal(t, redis.Nil, err, "Key harus sudah terhapus otomatis setelah TTL habis")
}

func TestStringSetNX(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	lockKey := UniqueKey("lock:order:1001")
	CleanKey(t, client, ctx, lockKey)

	// Fundamental SETNX (Set if Not eXists):
	// Berfungsi sebagai mekanisme Distributed Lock atau Idempotency Key.
	// Operasi ini bersifat atomik (tidak bisa disela request lain):
	// - Jika key BELUM ada -> Set berhasil, return true.
	// - Jika key SUDAH ada -> Ditolak, return false.
	// Selalu sertakan TTL agar jika sistem crash, lock tidak terkunci selamanya (deadlock).

	// Request 1: Mencoba mengambil lock pertama kali
	acquired, err := client.SetNX(ctx, lockKey, "instance-a", 5*time.Second).Result()
	assert.NoError(t, err)
	assert.True(t, acquired, "Request pertama harus sukses mengunci resource")

	// Request 2: Mencoba mengambil lock yang sama saat masih aktif
	acquiredAgain, err := client.SetNX(ctx, lockKey, "instance-b", 5*time.Second).Result()
	assert.NoError(t, err)
	assert.False(t, acquiredAgain, "Request kedua harus ditolak karena key sudah ada")
}

func TestStringIncrementAndDecrement(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	counterKey := UniqueKey("counter:views:video:42")
	CleanKey(t, client, ctx, counterKey)

	// Fundamental Atomic Increment / Decrement:
	// Meskipun Redis menyimpan angka sebagai string internal, Redis mampu mengonversinya
	// ke integer 64-bit bertanda secara on-the-fly untuk operasi INCR/DECR.
	// Sifatnya ATOMIK 100%: Ribuan request konkuren dapat memanggil INCR tanpa race condition!
	// Tidak perlu SELECT ... FOR UPDATE atau lock tabel database.

	// 1. INCR (menambah 1, jika belum ada key otomatis dimulai dari 0 -> 1)
	val1, err := client.Incr(ctx, counterKey).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), val1)

	// 2. INCRBY (menambah kelipatan tertentu, misal batch view +10)
	val2, err := client.IncrBy(ctx, counterKey, 10).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(11), val2)

	// 3. DECR (mengurangi 1)
	val3, err := client.Decr(ctx, counterKey).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(10), val3)

	// 4. DECRBY (mengurangi jumlah tertentu)
	val4, err := client.DecrBy(ctx, counterKey, 5).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(5), val4)
}

func TestStringMSetAndMGet(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	k1 := UniqueKey("mset:user:1")
	k2 := UniqueKey("mset:user:2")
	CleanKey(t, client, ctx, k1, k2)

	// Fundamental MSET & MGET:
	// Daripada mengirim N perintah GET/SET satu per satu yang memakan banyak round-trip TCP,
	// MSET dan MGET memungkinkan operasi banyak key sekaligus dalam 1 request round-trip jaringan.

	// 1. Simpan beberapa pasangan key-value sekaligus
	err := client.MSet(ctx, k1, "Adit", k2, "Budi").Err()
	assert.NoError(t, err)

	// 2. Ambil beberapa key sekaligus
	values, err := client.MGet(ctx, k1, k2, "key_yang_tidak_ada").Result()
	assert.NoError(t, err)
	assert.Len(t, values, 3)

	assert.Equal(t, "Adit", values[0])
	assert.Equal(t, "Budi", values[1])
	assert.Nil(t, values[2], "Key yang tidak ada bernilai nil di dalam slice hasil MGet")
}

func TestStringAppendAndStrLen(t *testing.T) {
	client, ctx := SetupTestClient(t)
	defer client.Close()

	key := UniqueKey("string:append")
	CleanKey(t, client, ctx, key)

	// 1. Set awal
	_ = client.Set(ctx, key, "Belajar", 0).Err()

	// 2. APPEND menambahkan string di akhir nilai yang sudah ada
	newLen, err := client.Append(ctx, key, " Golang").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(len("Belajar Golang")), newLen)

	// 3. STRLEN mengembalikan panjang karakter (byte length)
	strLen, err := client.StrLen(ctx, key).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(14), strLen)

	// 4. Verifikasi teks akhir
	finalVal, _ := client.Get(ctx, key).Result()
	assert.Equal(t, "Belajar Golang", finalVal)
}
