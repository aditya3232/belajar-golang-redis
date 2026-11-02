package belajar_golang_redis

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// GetRedisAddr mengembalikan alamat Redis dari environment variable REDIS_ADDR
// atau fallback ke default "localhost:6380".
func GetRedisAddr() string {
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		return addr
	}
	return "localhost:6380"
}

// GetRedisPassword mengembalikan password Redis dari environment variable REDIS_PASSWORD
// atau fallback ke default "password".
func GetRedisPassword() string {
	if pwd, exists := os.LookupEnv("REDIS_PASSWORD"); exists {
		return pwd
	}
	return "password"
}

// GetRedisClient membuat dan mengembalikan instance *redis.Client dengan konfigurasi
// connection pool yang optimal untuk aplikasi production.
//
// Fundamental Konfigurasi:
// 1. Addr: Alamat host:port server Redis.
// 2. Password: Diisi jika Redis mengaktifkan requirepass / autentikasi.
// 3. DB: Logical database index (0 - 15 secara default di Redis).
// 4. PoolSize: Jumlah maksimum koneksi TCP yang dibuka secara bersamaan.
//    Redis bekerja single-threaded di sisi CPU untuk command processing,
//    tetapi connection pooling di client Go mencegah overhead handshake TCP bolak-balik
//    saat melayani request konkuren dari banyak goroutine.
// 5. MinIdleConns: Jumlah koneksi menganggur (idle) yang tetap dipertahankan
//    agar selalu siap digunakan saat terjadi lonjakan traffic (traffic spike).
// 6. DialTimeout: Batas waktu maksimal saat inisiasi koneksi baru ke Redis.
// 7. ReadTimeout & WriteTimeout: Mencegah goroutine hanging jika Redis mengalami delay jaringan.
func GetRedisClient() *redis.Client {
	addr := GetRedisAddr()
	password := GetRedisPassword()

	dbIndex := 0
	if dbStr := os.Getenv("REDIS_DB"); dbStr != "" {
		if parsed, err := strconv.Atoi(dbStr); err == nil {
			dbIndex = parsed
		}
	}

	return redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           dbIndex,
		PoolSize:     10,              // Maksimal 10 koneksi konkuren
		MinIdleConns: 3,               // Minimal 3 koneksi idle selalu siaga
		DialTimeout:  5 * time.Second, // Timeout pembentukan socket TCP
		ReadTimeout:  3 * time.Second, // Timeout pembacaan respon dari Redis
		WriteTimeout: 3 * time.Second, // Timeout pengiriman perintah ke Redis
		MaxRetries:   2,               // Otomatis mencoba ulang jika terjadi transient network error
	})
}

// SetupTestClient adalah helper untuk unit test:
// Memastikan koneksi ke Redis aktif dengan perintah PING.
// Jika Redis belum berjalan, unit test akan memberikan pesan instruksi yang jelas.
func SetupTestClient(t *testing.T) (*redis.Client, context.Context) {
	t.Helper()
	client := GetRedisClient()
	ctx := context.Background()

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		t.Fatalf(
			"\n[REDIS BELUM AKTIF]\n"+
				"Gagal terhubung ke Redis di %s: %v\n"+
				"Pastikan Redis sudah berjalan dengan perintah:\n"+
				"  docker compose up -d\n"+
				"Atau tentukan alamat Redis melalui environment variable:\n"+
				"  export REDIS_ADDR=localhost:6379\n",
			GetRedisAddr(), err,
		)
	}

	return client, ctx
}

// CleanKey menghapus key setelah pengujian selesai untuk mencegah polusi data.
func CleanKey(t *testing.T, client *redis.Client, ctx context.Context, keys ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, key := range keys {
			_ = client.Del(ctx, key).Err()
		}
	})
}

// UniqueKey menghasilkan key unik dengan prefix tertentu agar tidak bentrok antar test.
func UniqueKey(prefix string) string {
	return fmt.Sprintf("test:%s:%d", prefix, time.Now().UnixNano())
}
