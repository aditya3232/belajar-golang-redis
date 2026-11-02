package belajar_golang_redis

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// TOPIC 1: REDIS CLIENT FUNDAMENTALS
// ============================================================================
// Fundamental:
// Redis adalah in-memory key-value data store berbasis single-threaded event loop
// (menggunakan I/O multiplexing seperti epoll di Linux).
// Library `go-redis/v9` mewajibkan penggunaan `context.Context` pada setiap operasi
// untuk mendukung tracing, deadline, timeout, dan cancellation secara idiomatik di Go.
//
// Connection Pooling:
// Go-redis secara otomatis mengelola connection pool di balik layar.
// Anda TIDAK PERLU membuat koneksi baru setiap kali ingin mengeksekusi perintah.
// Cukup buat satu instance `*redis.Client` (singleton) untuk digunakan bersama
// di seluruh aplikasi secara thread-safe (aman diakses banyak goroutine).
// ============================================================================

func TestClientPing(t *testing.T) {
	// 1. Inisialisasi client
	client, ctx := SetupTestClient(t)
	defer client.Close()

	// 2. Kirim perintah PING untuk memastikan server Redis hidup
	// Redis akan merespon dengan string "PONG".
	status := client.Ping(ctx)

	// Verifikasi tidak ada error dan hasil adalah "PONG"
	assert.NoError(t, status.Err(), "Ping ke Redis harus berhasil tanpa error")
	assert.Equal(t, "PONG", status.Val(), "Respon default dari PING adalah PONG")
}

func TestClientWithContextTimeout(t *testing.T) {
	// Fundamental Context Timeout:
	// Jika terjadi latensi jaringan atau Redis server overload, kita tidak ingin
	// request API di aplikasi backend kita menggantung (hanging) selamanya.
	// Dengan context timeout, perintah otomatis dibatalkan jika melebihi batas waktu.
	client, _ := SetupTestClient(t)
	defer client.Close()

	// Buat context dengan batas waktu sangat singkat (misal 500 milidetik)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	pong, err := client.Ping(ctx).Result()
	assert.NoError(t, err)
	assert.Equal(t, "PONG", pong)
}

func TestClientPoolStats(t *testing.T) {
	// Fundamental PoolStats:
	// PoolStats memberikan visibilitas terhadap kesehatan connection pool:
	// - TotalConns: Total koneksi yang sudah dibuat
	// - IdleConns: Koneksi menganggur yang siap dipakai
	// - ActiveConns: Koneksi yang sedang sibuk mengeksekusi perintah
	// - Misses: Berapa kali request harus menunggu koneksi baru karena pool penuh
	client, ctx := SetupTestClient(t)
	defer client.Close()

	// Eksekusi beberapa perintah untuk memicu penggunaan koneksi
	for i := 0; i < 5; i++ {
		_ = client.Ping(ctx).Err()
	}

	stats := client.PoolStats()
	assert.NotNil(t, stats)
	t.Logf("Pool Stats -> Total: %d, Idle: %d, Misses: %d, Timeouts: %d",
		stats.TotalConns, stats.IdleConns, stats.Misses, stats.Timeouts)
}

func TestClientDatabaseSelection(t *testing.T) {
	// Fundamental Logical Database:
	// Secara default, Redis menyediakan 16 database numerik (index 0 sampai 15).
	// Database index 0 adalah default.
	// Catatan: Pada arsitektur modern (terutama Redis Cluster), hanya database 0
	// yang didukung. Penggunaan database terpisah di satu instance sebaiknya
	// digantikan dengan namespacing key (misal: "app:user:1", "app:order:1").
	client := redis.NewClient(&redis.Options{
		Addr:     GetRedisAddr(),
		Password: GetRedisPassword(),
		DB:       1, // Menggunakan DB index 1
	})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := client.Ping(ctx).Err()
	assert.NoError(t, err, "Harus bisa terhubung ke logical DB 1")
}
